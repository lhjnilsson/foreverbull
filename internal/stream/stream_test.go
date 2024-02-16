package stream

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"
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
	dc := NewDependencyContainer()

	pool, err := pgxpool.New(context.Background(), environment.GetPostgresURL())
	suite.NoError(err)

	err = RecreateTables(context.Background(), pool)
	suite.NoError(err)

	suite.jt, err = NewJetstream()
	suite.NoError(err)

	stream, err := NewNATSStream(suite.jt, "test", dc, pool)
	suite.NoError(err)
	suite.stream = *stream.(*NATSStream)

	suite.stream.CommandSubscriber("return", "nil", ReturnNil)
	suite.stream.CommandSubscriber("return", "err", ReturnErr)
}

func (suite *NatsStreamTest) TearDownTest() {
	suite.stream.Unsubscribe()
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

func ReturnErr(ctx context.Context, message Message) error {
	return errors.New("test error")
}

func (test *NatsStreamTest) TestPubSub() {
	type TestCase struct {
		name           string
		message        message
		expectedStatus MessageStatus
	}
	testCases := []TestCase{
		{
			name:           "nil",
			message:        message{Module: "test", Component: "return", Method: "nil"},
			expectedStatus: MessageStatusComplete,
		},
		{
			name:           "err",
			message:        message{Module: "test", Component: "return", Method: "err"},
			expectedStatus: MessageStatusError,
		},
	}
	for _, tc := range testCases {
		test.Run(tc.name, func() {

			test.stream.Publish(context.Background(), &tc.message)
			time.Sleep(time.Second / 2)

			m, err := test.stream.repository.GetMessage(context.Background(), *tc.message.ID)
			test.NoError(err)
			fmt.Println(m.StatusHistory)
			test.Equal(tc.expectedStatus, m.StatusHistory[0].Status)
		})
	}
}

func (test *NatsStreamTest) TestRunOrchestration() {
	app := fx.New(
		fx.Provide(
			func() nats.JetStreamContext {
				return test.jt
			},
			func() *pgxpool.Pool {
				return test.stream.repository.db
			},
		),
		OrchestrationLifecycle,
	)
	test.Require().NoError(app.Start(context.Background()))

	test.Run("normal orchestration", func() {
		msg1, err := NewMessage("test", "return", "nil", TestPayload{Name: "test", Number: 1})
		test.NoError(err)
		msg2, err := NewMessage("test", "return", "nil", TestPayload{Name: "test", Number: 2})
		test.NoError(err)
		msg3, err := NewMessage("test", "return", "nil", TestPayload{Name: "test", Number: 3})
		test.NoError(err)

		orchestration := NewMessageOrchestration("test orchestration")
		orchestration.AddStep("step1", []Message{msg1})
		orchestration.AddStep("step2", []Message{msg2})
		orchestration.SettFallback([]Message{msg3})

		err = test.stream.RunOrchestration(context.Background(), orchestration)
		test.NoError(err)

		time.Sleep(time.Second / 2)

		m, err := test.stream.repository.GetMessage(context.Background(), msg1.GetID())
		test.NoError(err)
		test.Equal(MessageStatusComplete, m.StatusHistory[0].Status)
		m, err = test.stream.repository.GetMessage(context.Background(), msg2.GetID())
		test.NoError(err)
		test.Equal(MessageStatusComplete, m.StatusHistory[0].Status)
		m, err = test.stream.repository.GetMessage(context.Background(), msg3.GetID())
		test.NoError(err)
		test.Equal(MessageStatusCanceled, m.StatusHistory[0].Status)
	})
	test.Run("error orchestration", func() {
		msg1, err := NewMessage("test", "return", "err", TestPayload{Name: "test", Number: 1})
		test.NoError(err)
		msg2, err := NewMessage("test", "return", "nil", TestPayload{Name: "test", Number: 2})
		test.NoError(err)
		msg3, err := NewMessage("test", "return", "nil", TestPayload{Name: "test", Number: 3})
		test.NoError(err)

		orchestration := NewMessageOrchestration("test orchestration")
		orchestration.AddStep("step1", []Message{msg1})
		orchestration.AddStep("step2", []Message{msg2})
		orchestration.SettFallback([]Message{msg3})

		err = test.stream.RunOrchestration(context.Background(), orchestration)
		test.NoError(err)

		time.Sleep(time.Second / 2)

		m, err := test.stream.repository.GetMessage(context.Background(), msg1.GetID())
		test.NoError(err)
		test.Equal(MessageStatusError, m.StatusHistory[0].Status)
		m, err = test.stream.repository.GetMessage(context.Background(), msg2.GetID())
		test.NoError(err)
		test.Equal(MessageStatusCanceled, m.StatusHistory[0].Status)
		m, err = test.stream.repository.GetMessage(context.Background(), msg3.GetID())
		test.NoError(err)
		test.Equal(MessageStatusComplete, m.StatusHistory[0].Status)
	})

	test.NoError(app.Stop(context.Background()))
}
