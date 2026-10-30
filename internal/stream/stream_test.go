package stream_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/internal/stream/pb"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	common "github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/proto"
)

func ReturnNil(ctx context.Context, message stream.Message) error {
	return nil
}

func ReturnErr(ctx context.Context, message stream.Message) error {
	return errors.New("test error")
}

func ReturnNotice(ctx context.Context, message stream.Message) error {
	return &stream.Notice{Lines: []string{"Missing prices for XYZ on 2026-05-05, filled with the previous close. No trades on those days."}}
}

func ReturnProgress(ctx context.Context, message stream.Message) error {
	stream.ReportProgress(ctx, "Downloading prices: 0 of 2 symbols")
	stream.ReportProgress(ctx, "Downloading prices: 1 of 2 symbols")
	return nil
}

func Timeout(ctx context.Context, message stream.Message) error {
	done := make(chan bool)
	go func() {
		time.Sleep(time.Second * 3)
		done <- true
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type StreamTest struct {
	suite.Suite

	subscriber stream.Subscriber

	uut stream.NATSStream
}

func (test *StreamTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		NATS:     true,
		Postgres: true,
	})
}

func (test *StreamTest) SetupTest() {
	pool, err := pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)

	err = stream.RecreateTables(context.Background(), pool)
	test.Require().NoError(err)

	s, err := stream.NewNATSStream()
	test.Require().NoError(err)
	test.uut = *s.(*stream.NATSStream)

	test.subscriber, err = test.uut.NewSubscriber("test", stream.NewDependencyContainer())
	test.Require().NoError(err)

	test.Require().NoError(test.subscriber.Subscribe("return", "nil", ReturnNil))
	test.Require().NoError(test.subscriber.Subscribe("return", "err", ReturnErr))
	test.Require().NoError(test.subscriber.Subscribe("return", "timeout", Timeout))
	test.Require().NoError(test.subscriber.Subscribe("return", "notice", ReturnNotice))
	test.Require().NoError(test.subscriber.Subscribe("return", "progress", ReturnProgress))
}

func (test *StreamTest) TearDownTest() {
	test.NoError(test.subscriber.Unsubscribe())
}

func (test *StreamTest) parseStates(t *testing.T, msg stream.Message) []*pb.State {
	t.Helper()

	natsConnect, err := nats.Connect(environment.GetNATSURL())
	if err != nil {
		t.Fatal(err)
	}

	jetstream, err := natsConnect.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	sub, err := jetstream.PullSubscribe(msg.TaskStateTopic(), "")
	if err != nil {
		t.Fatal(err)
	}
	msgs, err := sub.Fetch(10)
	if err != nil {
		t.Fatal(err)
	}
	states := []*pb.State{}
	for _, msg := range msgs {
		state := &pb.State{}
		err := proto.Unmarshal(msg.Data, state)
		if err != nil {
			t.Fatal(err)
		}
		states = append(states, state)
	}
	return states
}

func TestStream(t *testing.T) {
	suite.Run(t, new(StreamTest))
}

func (test *StreamTest) TestPublish() {
	test.Run("WaitForCompletion returns a notice as a Notice", func() {
		msg, err := stream.NewMessage("test", "return", "notice", struct{}{})
		test.Require().NoError(err)
		err = test.uut.Publish(context.Background(), msg, stream.WaitForCompletion())
		var notice *stream.Notice
		test.Require().ErrorAs(err, &notice)
		test.Equal([]string{"Missing prices for XYZ on 2026-05-05, filled with the previous close. No trades on those days."}, notice.Lines)

		// The completed state carries the notice in notices, never in
		// error, so a reader that treats any error as a failure sees success.
		time.Sleep(time.Second)
		states := test.parseStates(test.T(), msg)
		test.Require().Len(states, 3)
		test.Require().Equal(pb.State_STATE_COMPLETED, states[2].State)
		test.Nil(states[2].Error)
		test.Equal([]string{"Missing prices for XYZ on 2026-05-05, filled with the previous close. No trades on those days."}, states[2].Notices)
	})
	test.Run("WaitForCompletion", func() {
		msg, err := stream.NewMessage("test", "return", "nil", struct{}{})
		test.Require().NoError(err)
		err = test.uut.Publish(context.Background(), msg, stream.WaitForCompletion())
		test.Require().NoError(err)

		time.Sleep(time.Second)
		states := test.parseStates(test.T(), msg)
		test.Require().Len(states, 3)
		test.Require().Contains([]pb.State_State{pb.State_STATE_RECEIVED, pb.State_STATE_PUBLISHED}, states[0].State)
		test.Require().Contains([]pb.State_State{pb.State_STATE_RECEIVED, pb.State_STATE_PUBLISHED}, states[1].State)
		test.Require().Equal(pb.State_STATE_COMPLETED, states[2].State)
	})
	test.Run("Timeout", func() {
		msg, err := stream.NewMessage("test", "return", "timeout", struct{}{})
		test.Require().NoError(err)
		ctx := context.Background()
		ctx, cancel := context.WithCancel(ctx)
		errCh := make(chan error)
		go func() {
			errCh <- test.uut.Publish(ctx, msg, stream.WaitForCompletion())
		}()
		time.Sleep(time.Second)
		cancel()
		err = <-errCh
		test.Require().Error(err)
		test.Require().ErrorContains(err, "canceled")
		time.Sleep(time.Second)
		states := test.parseStates(test.T(), msg)

		test.Require().Len(states, 3)
		test.Require().Contains([]pb.State_State{pb.State_STATE_RECEIVED, pb.State_STATE_PUBLISHED}, states[0].State)
		test.Require().Contains([]pb.State_State{pb.State_STATE_RECEIVED, pb.State_STATE_PUBLISHED}, states[1].State)
		test.Require().Equal(pb.State_STATE_CANCELED, states[2].State)
	})
}

func (test *StreamTest) TestRunOrchestration() {
	successfulMsg, err := stream.NewMessage("test", "return", "nil", struct{}{})
	test.Require().NoError(err)
	errorMsg, err := stream.NewMessage("test", "return", "err", struct{}{})
	test.Require().NoError(err)
	timeoutMsg, err := stream.NewMessage("test", "return", "timeout", struct{}{})
	test.Require().NoError(err)

	test.Run("Successful", func() {
		o := pb.Orchestration{
			Steps: []*pb.Orchestration_Step{
				{
					Num: int32(1),
					Messages: []*pb.Message{
						successfulMsg.GetMsg(), successfulMsg.GetMsg(),
					},
				},
			},
		}

		stepCh := test.uut.RunOrchestration(context.Background(), &o)
		steps := []*pb.Orchestration_Step{}
		for st := range stepCh {
			steps = append(steps, st)
		}
		test.Require().Len(steps, 2)
		test.Require().Equal(common.State_STATE_RUNNING, steps[0].State)
		test.Require().Equal(common.State_STATE_COMPLETED, steps[1].State)
	})
	test.Run("Notice completes the step and carries it in Notices", func() {
		noticeMsg, err := stream.NewMessage("test", "return", "notice", struct{}{})
		test.Require().NoError(err)
		o := pb.Orchestration{
			Steps: []*pb.Orchestration_Step{
				{Num: int32(1), Messages: []*pb.Message{noticeMsg.GetMsg(), successfulMsg.GetMsg()}},
				{Num: int32(2), Messages: []*pb.Message{successfulMsg.GetMsg()}},
			},
		}

		steps := []*pb.Orchestration_Step{}
		for st := range test.uut.RunOrchestration(context.Background(), &o) {
			steps = append(steps, st)
		}
		test.Require().Len(steps, 4)
		test.Require().Equal(common.State_STATE_COMPLETED, steps[1].State)
		// The notice has its own field; Error stays nil on a completed step,
		// so a reader that treats any Error as a failure still sees success.
		test.Nil(steps[1].Error)
		test.Equal([]string{"Missing prices for XYZ on 2026-05-05, filled with the previous close. No trades on those days."}, steps[1].Notices)
		test.Require().Equal(common.State_STATE_COMPLETED, steps[3].State)
		test.Nil(steps[3].Error)
		test.Empty(steps[3].Notices)
	})
	test.Run("Progress is a running step with the text, in order", func() {
		progressMsg, err := stream.NewMessage("test", "return", "progress", struct{}{})
		test.Require().NoError(err)
		o := pb.Orchestration{
			Steps: []*pb.Orchestration_Step{
				{Num: int32(2), Messages: []*pb.Message{progressMsg.GetMsg()}},
			},
		}

		steps := []*pb.Orchestration_Step{}
		for st := range test.uut.RunOrchestration(context.Background(), &o) {
			steps = append(steps, st)
		}
		test.Require().Len(steps, 4)
		test.Equal(common.State_STATE_RUNNING, steps[0].State)
		test.Nil(steps[0].Error)
		for index, want := range []string{"Downloading prices: 0 of 2 symbols", "Downloading prices: 1 of 2 symbols"} {
			test.Equal(int32(2), steps[index+1].Num)
			test.Equal(common.State_STATE_RUNNING, steps[index+1].State)
			test.Require().NotNil(steps[index+1].Error)
			test.Equal(want, *steps[index+1].Error)
		}
		test.Equal(common.State_STATE_COMPLETED, steps[3].State)
		test.Nil(steps[3].Error, "progress is not a notice")
	})
	test.Run("Error", func() {
		o := pb.Orchestration{
			Steps: []*pb.Orchestration_Step{
				{
					Num: int32(1),
					Messages: []*pb.Message{
						successfulMsg.GetMsg(), errorMsg.GetMsg(),
					},
				},
				{
					Num: int32(2),
					Messages: []*pb.Message{
						successfulMsg.GetMsg(),
					},
				},
			},
		}

		stepCh := test.uut.RunOrchestration(context.Background(), &o)
		steps := []*pb.Orchestration_Step{}
		for st := range stepCh {
			steps = append(steps, st)
		}
		test.Require().Len(steps, 2)
		test.Require().Equal(common.State_STATE_RUNNING, steps[0].State)
		test.Require().Equal(common.State_STATE_FAILED, steps[1].State)
		test.Require().Contains(*steps[1].Error, "test error")
	})
	test.Run("Cancel", func() {
		o := pb.Orchestration{
			Steps: []*pb.Orchestration_Step{
				{
					Num: int32(1),
					Messages: []*pb.Message{
						successfulMsg.GetMsg(), timeoutMsg.GetMsg(),
					},
				},
				{
					Num: int32(2),
					Messages: []*pb.Message{
						successfulMsg.GetMsg(),
					},
				},
			},
		}

		ctx := context.Background()
		ctx, cancel := context.WithCancel(ctx)

		// The collector hands the steps over once RunOrchestration closes
		// its channel, so the test never reads the slice while it grows.
		done := make(chan []*pb.Orchestration_Step, 1)
		go func() {
			steps := []*pb.Orchestration_Step{}
			for st := range test.uut.RunOrchestration(ctx, &o) {
				steps = append(steps, st)
			}
			done <- steps
		}()
		time.Sleep(time.Second)
		cancel()
		var steps []*pb.Orchestration_Step
		select {
		case steps = <-done:
		case <-time.After(5 * time.Second):
			test.FailNow("orchestration did not stop after cancel")
		}
		test.Require().Len(steps, 2)
		test.Require().Equal(common.State_STATE_RUNNING, steps[0].State)
		test.Require().Equal(common.State_STATE_FAILED, steps[1].State)
		test.Require().Contains(*steps[1].Error, "canceled")
	})
}
