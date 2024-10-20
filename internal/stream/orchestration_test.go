package stream

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"
)

type OrchestrationTest struct {
	suite.Suite

	conn *pgxpool.Pool

	nc     *nats.Conn
	jt     nats.JetStreamContext
	stream NATSStream

	orchestration *fx.App
}

func (test *OrchestrationTest) SetupTest() {
	var err error

	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
		NATS:     true,
	})

	test.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)

	err = RecreateTables(context.Background(), test.conn)
	test.Require().NoError(err)

	test.nc, test.jt, err = New()
	test.Require().NoError(err)

	test.orchestration = fx.New(
		fx.Provide(
			func() nats.JetStreamContext {
				return test.jt
			},
			func() *pgxpool.Pool {
				return test.conn
			},
		),
		OrchestrationLifecycle,
	)
	err = test.orchestration.Start(context.Background())
	test.Require().NoError(err)
}

func (test *OrchestrationTest) TearDownTest() {
	err := test.stream.Unsubscribe()
	test.Require().NoError(err)

	err = test.orchestration.Stop(context.Background())
	test.Require().NoError(err)

	test.nc.Close()
}

func TestOrchestration(t *testing.T) {
	suite.Run(t, new(OrchestrationTest))
}

func (test *OrchestrationTest) TestMessageHandler() {
	repository := repository{db: test.conn}
	createOrchestration := func(t *testing.T, orchestration *MessageOrchestration) {
		t.Helper()

		for _, step := range orchestration.Steps {
			for _, cmd := range step.Commands {
				msg := cmd.(*message)
				err := repository.CreateMessage(context.Background(), msg)
				test.Require().NoError(err)
			}
		}

		for _, cmd := range orchestration.FallbackStep.Commands {
			msg := cmd.(*message)
			err := repository.CreateMessage(context.Background(), msg)
			test.Require().NoError(err)
		}
	}

	test.Run("test successful", func() {
		orchestration := NewMessageOrchestration("test orchestration")
		msg1, err := NewMessage("test_module", "test_comp", "test_method", nil)
		test.Require().NoError(err)
		orchestration.AddStep("first step", []Message{msg1})

		msg2, err := NewMessage("test_module", "test_comp", "test_method", nil)
		test.Require().NoError(err)
		orchestration.AddStep("second step", []Message{msg2})

		fallbackMsg, err := NewMessage("test_module", "test_comp", "test_method", nil)
		test.Require().NoError(err)
		orchestration.SettFallback([]Message{fallbackMsg})

		createOrchestration(test.T(), orchestration)

		test.Require().NoError(repository.UpdateMessageStatus(context.Background(), msg1.GetID(), MessageStatusComplete, nil))
		payload, err := json.Marshal(msg1)
		test.Require().NoError(err)
		_, err = test.jt.Publish("foreverbull.test_module.test_comp.test_method.event", payload)
		test.Require().NoError(err)
		time.Sleep(time.Second / 2) // wait for message to be processed

		msg, err := repository.GetMessage(context.Background(), msg2.GetID())
		test.Require().NoError(err)
		test.Equal(MessageStatusPublished, msg.StatusHistory[0].Status)

		test.NoError(repository.UpdateMessageStatus(context.Background(), msg2.GetID(), MessageStatusComplete, nil))
		payload, err = json.Marshal(msg2)
		test.Require().NoError(err)
		_, err = test.jt.Publish("foreverbull.test_module.test_comp.test_method.event", payload)
		test.Require().NoError(err)
		time.Sleep(time.Second / 2) // wait for message to be processed

		msg, err = repository.GetMessage(context.Background(), fallbackMsg.GetID())
		test.Require().NoError(err)
		test.Equal(MessageStatusCanceled, msg.StatusHistory[0].Status)
	})

	test.Run("expect fallback command", func() {
		orchestration := NewMessageOrchestration("test orchestration")
		msg1, err := NewMessage("test_module", "test_comp", "test_method", nil)
		test.Require().NoError(err)
		orchestration.AddStep("first step", []Message{msg1})

		msg2, err := NewMessage("test_module", "test_comp", "test_method", nil)
		test.Require().NoError(err)
		orchestration.AddStep("second step", []Message{msg2})

		fallbackMsg, err := NewMessage("test_module", "test_comp", "test_method", nil)
		test.Require().NoError(err)
		orchestration.SettFallback([]Message{fallbackMsg})

		createOrchestration(test.T(), orchestration)

		test.NoError(repository.UpdateMessageStatus(context.Background(), msg1.GetID(), MessageStatusError, nil))
		payload, err := json.Marshal(msg1)
		test.Require().NoError(err)
		_, err = test.jt.Publish("foreverbull.test_module.test_comp.test_method.event", payload)
		test.Require().NoError(err)
		time.Sleep(time.Second / 2) // wait for message to be processed

		msg, err := repository.GetMessage(context.Background(), msg2.GetID())
		test.Require().NoError(err)
		test.Equal(MessageStatusCanceled, msg.StatusHistory[0].Status)

		msg, err = repository.GetMessage(context.Background(), fallbackMsg.GetID())
		test.Require().NoError(err)
		test.Equal(MessageStatusPublished, msg.StatusHistory[0].Status)
	})
}
