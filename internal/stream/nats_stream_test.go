package stream

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"
)

type NatsStreamTest struct {
	suite.Suite

	jt     nats.JetStreamContext
	stream NATSStream
}

func (suite *NatsStreamTest) SetupTest() {
	helper.SetupEnvironment(suite.T(), &helper.Containers{
		Postgres: true,
		NATS:     true,
	})

	log := zaptest.NewLogger(suite.T())

	dc := NewDependencyContainer()

	pool, err := pgxpool.New(context.Background(), environment.GetPostgresURL())
	suite.NoError(err)

	err = RecreateTables(context.Background(), pool)
	suite.NoError(err)

	suite.jt, err = NewJetstream(environment.GetNATSURL())
	suite.NoError(err)

	stream, err := NewNATSStream(suite.jt, "test", log, dc, pool)
	suite.NoError(err)
	suite.stream = *stream.(*NATSStream)
}

func TestNatStream(t *testing.T) {
	suite.Run(t, new(NatsStreamTest))
}

type TestPayload struct {
	Name   string `json:"name"`
	Number int    `json:"number"`
	Object struct {
		At time.Time `json:"at"`
	} `json:"object"`
}

func ReturnNil(ctx context.Context, message Message) error {
	return nil
}

func (suite *NatsStreamTest) TestReturnNil() {
	payload := TestPayload{Name: "test", Number: 1}
	payload.Object.At = time.Now().UTC().Round(0)
	payloadBytes, err := json.Marshal(payload)
	suite.NoError(err)

	msg := &message{
		Module:    "test",
		Component: "test",
		Method:    "test",
		Payload:   payloadBytes,
	}

	m, err := suite.stream.CreateMessage(context.Background(), msg)
	suite.NoError(err)
	msg = m.(*message)
	suite.NotNil(msg.ID)
	suite.Equal(MessageStatusCreated, msg.StatusHistory[0].Status)
	suite.NotNil(msg.StatusHistory[0].OccurredAt)

	err = suite.stream.CommandSubscriber("test", "test", ReturnNil)
	suite.NoError(err)

	err = suite.stream.Publish(context.Background(), msg)
	suite.NoError(err)

	time.Sleep(time.Second)

	msg, err = suite.stream.repository.GetMessage(context.Background(), *msg.ID)
	suite.NoError(err)

	storedPayload := TestPayload{}
	err = json.Unmarshal(msg.Payload, &storedPayload)
	suite.NoError(err)
	suite.Equal(payload, storedPayload)

	suite.Len(msg.StatusHistory, 4)

	suite.Equal(MessageStatusComplete, msg.StatusHistory[0].Status)
	suite.NotNil(msg.StatusHistory[0].OccurredAt)
	suite.Nil(msg.StatusHistory[0].Error)

	suite.Equal(MessageStatusReceived, msg.StatusHistory[1].Status)
	suite.NotNil(msg.StatusHistory[1].OccurredAt)
	suite.Nil(msg.StatusHistory[1].Error)

	suite.Equal(MessageStatusPublished, msg.StatusHistory[2].Status)
	suite.NotNil(msg.StatusHistory[2].OccurredAt)
	suite.Nil(msg.StatusHistory[2].Error)

	suite.Equal(MessageStatusCreated, msg.StatusHistory[3].Status)
	suite.NotNil(msg.StatusHistory[3].OccurredAt)
	suite.Nil(msg.StatusHistory[3].Error)

	err = suite.stream.Unsubscribe()
	suite.NoError(err)
}

func ReturnErr(ctx context.Context, message Message) error {
	return errors.New("test error")
}

func (suite *NatsStreamTest) TestReturnErr() {
	payload := TestPayload{Name: "test", Number: 1}
	payload.Object.At = time.Now().UTC().Round(0)
	payloadBytes, err := json.Marshal(payload)
	suite.NoError(err)

	msg := &message{
		Module:    "test",
		Component: "test",
		Method:    "test",
		Payload:   payloadBytes,
	}

	m, err := suite.stream.CreateMessage(context.Background(), msg)
	suite.NoError(err)
	msg = m.(*message)
	suite.NotNil(msg.ID)
	suite.Equal(MessageStatusCreated, msg.StatusHistory[0].Status)
	suite.NotNil(msg.StatusHistory[0].OccurredAt)

	err = suite.stream.CommandSubscriber("test", "test", ReturnErr)
	suite.NoError(err)

	err = suite.stream.Publish(context.Background(), msg)
	suite.NoError(err)

	time.Sleep(time.Second)

	msg, err = suite.stream.repository.GetMessage(context.Background(), *msg.ID)
	suite.NoError(err)

	storedPayload := TestPayload{}
	err = json.Unmarshal(msg.Payload, &storedPayload)
	suite.NoError(err)
	suite.Equal(payload, storedPayload)

	suite.Len(msg.StatusHistory, 4)

	suite.Equal(MessageStatusComplete, msg.StatusHistory[0].Status)
	suite.NotNil(msg.StatusHistory[0].OccurredAt)
	suite.NotNil(msg.StatusHistory[0].Error)
	suite.Equal("test error", *msg.StatusHistory[0].Error)

	suite.Equal(MessageStatusReceived, msg.StatusHistory[1].Status)
	suite.NotNil(msg.StatusHistory[1].OccurredAt)
	suite.Nil(msg.StatusHistory[1].Error)

	suite.Equal(MessageStatusPublished, msg.StatusHistory[2].Status)
	suite.NotNil(msg.StatusHistory[2].OccurredAt)
	suite.Nil(msg.StatusHistory[2].Error)

	suite.Equal(MessageStatusCreated, msg.StatusHistory[3].Status)
	suite.NotNil(msg.StatusHistory[3].OccurredAt)
	suite.Nil(msg.StatusHistory[3].Error)

	err = suite.stream.Unsubscribe()
	suite.NoError(err)
}

func (suite *NatsStreamTest) TestOrchestration() {
	o := NewMessageOrchestration("test orchestration")
	suite.NotNil(o.OrchestrationID)

	payload := TestPayload{Name: "payload", Number: 1}
	payload.Object.At = time.Now().Round(0)
	payloadBytes, err := json.Marshal(payload)
	suite.NoError(err)

	o.AddStep("step one", []Message{&message{Module: "msg1", Component: "msg1", Method: "msg1", Payload: payloadBytes}})
	o.AddStep("step two", []Message{
		&message{Module: "msg2", Component: "msg2", Method: "msg2", Payload: payloadBytes},
		&message{Module: "msg3", Component: "msg3", Method: "msg3", Payload: payloadBytes}})
	o.SettFallback([]Message{&message{Module: "msg4", Component: "msg4", Method: "msg4", Payload: payloadBytes}})
	err = suite.stream.CreateOrchestration(context.Background(), o)
	suite.NoError(err)

	running, err := suite.stream.repository.OrchestrationIsRunning(context.Background(), o.OrchestrationID)
	suite.NoError(err)
	suite.False(running)

	msgs, err := suite.stream.repository.GetLatestUnpublishedOrchestrationStepCommands(context.Background(), o.OrchestrationID)
	suite.NoError(err)
	suite.Len(*msgs, 1)

	complete, err := suite.stream.repository.OrchestrationIsComplete(context.Background(), o.OrchestrationID)
	suite.NoError(err)
	suite.False(complete)

	complete, err = suite.stream.repository.OrchestrationStepIsComplete(context.Background(), o.OrchestrationID, "step one")
	suite.NoError(err)
	suite.False(complete)

	for _, msg := range *msgs {
		err = suite.stream.repository.UpdateMessageStatus(context.Background(), *msg.ID, MessageStatusPublished, nil)
		suite.NoError(err)

		err = suite.stream.repository.UpdateMessageStatus(context.Background(), *msg.ID, MessageStatusReceived, nil)
		suite.NoError(err)

		err = suite.stream.repository.UpdateMessageStatus(context.Background(), *msg.ID, MessageStatusComplete, nil)
		suite.NoError(err)
	}

	running, err = suite.stream.repository.OrchestrationIsRunning(context.Background(), o.OrchestrationID)
	suite.NoError(err)
	suite.True(running)

	complete, err = suite.stream.repository.OrchestrationIsComplete(context.Background(), o.OrchestrationID)
	suite.NoError(err)
	suite.False(complete)

	complete, err = suite.stream.repository.OrchestrationStepIsComplete(context.Background(), o.OrchestrationID, "step one")
	suite.NoError(err)
	suite.True(complete)

	msgs, err = suite.stream.repository.GetLatestUnpublishedOrchestrationStepCommands(context.Background(), o.OrchestrationID)
	suite.NoError(err)
	suite.Len(*msgs, 2)
}
