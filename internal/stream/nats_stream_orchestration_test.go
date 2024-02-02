package stream

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

type CommandPayload struct {
}

func CommandReturnNil(ctx context.Context, message Message) error {
	return nil
}

func CommandReturnError(ctx context.Context, message Message) error {
	return fmt.Errorf("error")
}

type OrchestrationTest struct {
	suite.Suite

	conn *pgxpool.Pool

	jt     nats.JetStreamContext
	stream NATSStream

	orchestration *fx.App
}

func (test *OrchestrationTest) SetupTest() {
	var err error

	helper.SetupEnvironment(test.T(), &helper.Containers{
		Postgres: true,
		NATS:     true,
	})
	log := zaptest.NewLogger(test.T())
	test.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.NoError(err)

	err = RecreateTables(context.Background(), test.conn)
	test.NoError(err)

	test.jt, err = NewJetstream(environment.GetNATSURL())
	test.NoError(err)

	dc := NewDependencyContainer()
	stream, err := NewNATSStream(test.jt, "orchestration_test", log, dc, test.conn)
	test.NoError(err)
	test.stream = *stream.(*NATSStream)

	err = test.stream.CommandSubscriber("return", "nil", CommandReturnNil)
	test.NoError(err)

	err = test.stream.CommandSubscriber("return", "err", CommandReturnError)
	test.NoError(err)

	test.orchestration = fx.New(
		fx.Provide(
			func() nats.JetStreamContext {
				return test.jt
			},
			func() *zap.Logger {
				return log
			},
			func() *pgxpool.Pool {
				return test.conn
			},
		),
		OrchestrationLifecycle,
	)
	err = test.orchestration.Start(context.Background())
	test.NoError(err)
}

func (test *OrchestrationTest) TearDownTest() {
	err := test.stream.Unsubscribe()
	test.NoError(err)

	err = test.orchestration.Stop(context.Background())
	test.NoError(err)
}

func TestNATSOrchestration(t *testing.T) {
	suite.Run(t, new(OrchestrationTest))
}

func (test *OrchestrationTest) TestNewMessageOrchestration() {
	test.Run("fail to create, missing fallback", func() {
		o := NewMessageOrchestration("fail to create, missing fallback")
		err := test.stream.CreateOrchestration(context.Background(), o)
		test.Error(err)
		test.EqualError(err, "orchestration must have fallback step")
	})

	test.Run("successful orchestration", func() {
		o := NewMessageOrchestration("successful orchestration")

		msg1, err := NewMessage("orchestration_test", "return", "nil", CommandPayload{})
		test.NoError(err)
		msg2, err := NewMessage("orchestration_test", "return", "nil", CommandPayload{})
		test.NoError(err)
		o.AddStep("step one", []Message{msg1, msg2})

		msg1, err = NewMessage("orchestration_test", "return", "nil", CommandPayload{})
		test.NoError(err)
		msg2, err = NewMessage("orchestration_test", "return", "nil", CommandPayload{})
		test.NoError(err)
		o.AddStep("step two", []Message{msg1, msg2})

		msg, err := NewMessage("orchestration_test", "return", "nil", CommandPayload{})
		test.NoError(err)
		o.SettFallback([]Message{msg})

		err = test.stream.CreateOrchestration(context.Background(), o)
		test.NoError(err)

		err = test.stream.RunOrchestration(context.Background(), o.OrchestrationID)
		test.NoError(err)

		test.NoError(helper.WaitUntilCondition(test.T(), func() (bool, error) {
			return test.stream.repository.OrchestrationIsComplete(context.Background(), o.OrchestrationID)
		}, time.Second))

		running, err := test.stream.repository.OrchestrationIsRunning(context.Background(), o.OrchestrationID)
		test.NoError(err)
		test.False(running)

		complete, err := test.stream.repository.OrchestrationIsComplete(context.Background(), o.OrchestrationID)
		test.NoError(err)
		test.True(complete)

		var fallbackStatus string
		err = test.conn.QueryRow(context.Background(),
			`SELECT status FROM message WHERE message.orchestration_id=$1 AND message.orchestration_step=$2 LIMIT 1`,
			o.OrchestrationID, "fallback").Scan(&fallbackStatus)
		test.NoError(err)
		test.Equal(string(MessageStatusCanceled), fallbackStatus)
	})

	test.Run("failed orchestration", func() {
		o := NewMessageOrchestration("failing orchestration")

		msg, err := NewMessage("orchestration_test", "return", "nil", CommandPayload{})
		test.NoError(err)
		o.AddStep("step one", []Message{msg})

		msg, err = NewMessage("orchestration_test", "return", "err", CommandPayload{})
		test.NoError(err)
		o.AddStep("step two", []Message{msg})

		msg, err = NewMessage("orchestration_test", "return", "nil", CommandPayload{})
		test.NoError(err)
		o.AddStep("step three", []Message{msg})

		msg, err = NewMessage("orchestration_test", "return", "nil", CommandPayload{})
		test.NoError(err)
		o.SettFallback([]Message{msg})

		err = test.stream.CreateOrchestration(context.Background(), o)
		test.NoError(err)

		err = test.stream.RunOrchestration(context.Background(), o.OrchestrationID)
		test.NoError(err)

		test.NoError(helper.WaitUntilCondition(test.T(), func() (bool, error) {
			return test.stream.repository.OrchestrationIsComplete(context.Background(), o.OrchestrationID)
		}, time.Second))

		running, err := test.stream.repository.OrchestrationIsRunning(context.Background(), o.OrchestrationID)
		test.NoError(err)
		test.False(running)

		complete, err := test.stream.repository.OrchestrationIsComplete(context.Background(), o.OrchestrationID)
		test.NoError(err)
		test.True(complete)

		var status string
		err = test.conn.QueryRow(context.Background(),
			`SELECT status FROM message WHERE message.orchestration_id=$1 AND message.orchestration_step=$2 LIMIT 1`,
			o.OrchestrationID, "step three").Scan(&status)
		test.NoError(err)
		test.Equal(string(MessageStatusCanceled), status)

		err = test.conn.QueryRow(context.Background(),
			`SELECT status FROM message WHERE message.orchestration_id=$1 AND message.orchestration_step=$2 LIMIT 1`,
			o.OrchestrationID, "fallback").Scan(&status)
		test.NoError(err)
		test.Equal(string(MessageStatusComplete), status)
	})
}
