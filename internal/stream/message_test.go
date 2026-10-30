package stream

import (
	"context"
	"testing"

	"github.com/lhjnilsson/foreverbull/internal/stream/pb"
	commonPb "github.com/lhjnilsson/foreverbull/pkg/pb"
	backtestServicePb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest/service"
	"github.com/stretchr/testify/suite"
)

type MessageTest struct {
	suite.Suite
}

func (test *MessageTest) SetupTest() {
}

func TestMessage(t *testing.T) {
	suite.Run(t, new(MessageTest))
}

type DemoEntity struct {
	Key   string `json:"key"`
	Value int    `json:"value"`
}

func (test *MessageTest) TestNewMessage() {
	test.Run("successful", func() {
		msg, err := NewMessage("module", "component", "method", DemoEntity{Key: "key", Value: 1})
		test.Require().NoError(err)
		test.NotNil(msg)

		parsed, ok := msg.(*message)
		test.True(ok)
		test.NotNil(parsed.msg.Id)
		test.Nil(parsed.msg.OrchestrationName)
		test.Nil(parsed.msg.OrchestrationId)
		test.Nil(parsed.msg.OrchestrationStep)
		test.Nil(parsed.msg.OrchestrationStepNumber)
		test.Nil(parsed.msg.OrchestrationFallbackStep)

		test.Equal("module", parsed.msg.Module)
		test.Equal("component", parsed.msg.Component)
		test.Equal("method", parsed.msg.Method)
		test.JSONEq(`{"key":"key","value":1}`, string(parsed.msg.Payload))
		test.Nil(parsed.dependencyContainer)
	})
	test.Run("bad entity", func() {
		msg, err := NewMessage("module", "component", "method", func() {})
		test.Require().Error(err)
		test.Nil(msg)
	})
}

type DependencyContainerTest struct {
	suite.Suite
}

func (test *DependencyContainerTest) SetupTest() {
}

func TestDependencyContainer(t *testing.T) {
	suite.Run(t, new(DependencyContainerTest))
}

func (test *DependencyContainerTest) TestNewDependencyContainer() {
	test.Run("create and add", func() {
		container := NewDependencyContainer()
		test.NotNil(container)
	})
	test.Run("call", func() {
		container := NewDependencyContainer()
		test.NotNil(container)

		method := func(_ context.Context, _ Message) (interface{}, error) { return "working", nil }
		container.AddMethod("key", method)

		msg := &message{dependencyContainer: container.(*dependencyContainer)}
		_, err := msg.Call(context.Background(), "key")
		test.Require().NoError(err)
	})
	test.Run("call missing", func() {
		defer func() {
			if r := recover(); r != nil {
				test.Equal("dependency not found: key", r)
			}
		}()

		container := NewDependencyContainer()
		test.NotNil(container)

		msg := &message{dependencyContainer: container.(*dependencyContainer)}
		_, _ = msg.Call(context.Background(), "key")
	})
	test.Run("MustGet", func() {
		container := NewDependencyContainer()
		test.NotNil(container)

		singleton := "test_value"
		container.AddSingleton("key", singleton)

		msg := &message{dependencyContainer: container.(*dependencyContainer)}
		value := msg.MustGet("key")
		test.Equal(singleton, value)
	})
	test.Run("MustGet missing", func() {
		defer func() {
			if r := recover(); r != nil {
				test.Equal("dependency not found: key", r)
			}
		}()

		container := NewDependencyContainer()
		test.NotNil(container)

		msg := &message{dependencyContainer: container.(*dependencyContainer)}
		_ = msg.MustGet("key")
	})
}

type UpdateOrchestrationTest struct {
	suite.Suite

	msg *backtestServicePb.CreateBacktestResponse
}

func (test *UpdateOrchestrationTest) SetupSubTest() {
	test.msg = &backtestServicePb.CreateBacktestResponse{
		CREATING: &commonPb.Step{
			State: commonPb.State_STATE_UNSPECIFIED,
		},
		DOWNLOADING: &commonPb.Step{
			State: commonPb.State_STATE_UNSPECIFIED,
		},
		INGESTING: &commonPb.Step{
			State: commonPb.State_STATE_UNSPECIFIED,
		},
		STORING: &commonPb.Step{
			State: commonPb.State_STATE_UNSPECIFIED,
		},
		Backtest: nil,
	}
}

func TestUpdateOrchestration(t *testing.T) {
	suite.Run(t, new(UpdateOrchestrationTest))
}

func (test *UpdateOrchestrationTest) TestUpdateOrchestration() {
	test.Run("mark as running", func() {
		step := &pb.Orchestration_Step{
			Num:   1,
			State: commonPb.State_STATE_RUNNING,
		}
		err := UpdateOrchestration(test.msg, step)
		test.Require().NoError(err)
		test.Equal(commonPb.State_STATE_RUNNING, test.msg.CREATING.State)
	})
	test.Run("mark as completed", func() {
		step := &pb.Orchestration_Step{
			Num:   1,
			State: commonPb.State_STATE_COMPLETED,
		}
		err := UpdateOrchestration(test.msg, step)
		test.Require().NoError(err)
		test.Equal(commonPb.State_STATE_COMPLETED, test.msg.CREATING.State)
	})
	test.Run("mark as completed with notices", func() {
		lines := []string{
			"Missing prices for ABC on 2026-05-05, filled with the previous close. No trades on those days.",
			"Missing prices for XYZ on 2026-05-05, filled with the previous close. No trades on those days.",
		}
		step := &pb.Orchestration_Step{
			Num:     3,
			State:   commonPb.State_STATE_COMPLETED,
			Notices: lines,
		}
		err := UpdateOrchestration(test.msg, step)
		test.Require().NoError(err)
		test.Equal(commonPb.State_STATE_COMPLETED, test.msg.INGESTING.State)
		test.Equal(lines, test.msg.INGESTING.Notices)
		// Message is the failure text; a completed step leaves it empty.
		test.Empty(test.msg.INGESTING.Message)
	})
	test.Run("completing after progress clears the progress text", func() {
		progress := "Downloading prices: 1,240 of 10,000 symbols"
		test.Require().NoError(UpdateOrchestration(test.msg, &pb.Orchestration_Step{
			Num: 2, State: commonPb.State_STATE_RUNNING, Error: &progress,
		}))
		test.Equal(commonPb.State_STATE_RUNNING, test.msg.DOWNLOADING.State)
		test.Equal(progress, test.msg.DOWNLOADING.Message)

		test.Require().NoError(UpdateOrchestration(test.msg, &pb.Orchestration_Step{
			Num: 2, State: commonPb.State_STATE_COMPLETED,
		}))
		test.Equal(commonPb.State_STATE_COMPLETED, test.msg.DOWNLOADING.State)
		test.Empty(test.msg.DOWNLOADING.Message, "a leftover progress text would print as a notice")
	})
	test.Run("mark as failed", func() {
		step := &pb.Orchestration_Step{
			Num:   1,
			State: commonPb.State_STATE_FAILED,
			Error: func() *string { s := "example error"; return &s }(),
		}
		err := UpdateOrchestration(test.msg, step)
		test.Require().NoError(err)
		test.Equal(commonPb.State_STATE_FAILED, test.msg.CREATING.State)
		test.Equal("example error", test.msg.CREATING.Message)
	})
}
