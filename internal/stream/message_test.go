package stream

import (
	"context"
	"testing"

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
		test.NoError(err)
		test.NotNil(msg)

		parsed, ok := msg.(*message)
		test.True(ok)
		test.Nil(parsed.ID)
		test.Nil(parsed.OrchestrationName)
		test.Nil(parsed.OrchestrationID)
		test.Nil(parsed.OrchestrationStep)
		test.Nil(parsed.OrchestrationStepNumber)
		test.Nil(parsed.OrchestrationFallbackStep)

		test.Equal("module", parsed.Module)
		test.Equal("component", parsed.Component)
		test.Equal("method", parsed.Method)
		test.Nil(parsed.Error)
		test.Equal([]byte(`{"key":"key","value":1}`), parsed.Payload)
		test.Nil(parsed.StatusHistory)
		test.Nil(parsed.dependencyContainer)
	})
	test.Run("bad entity", func() {
		msg, err := NewMessage("module", "component", "method", func() {})
		test.Error(err)
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
		test.NoError(err)
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

type MessageOrchestrationTest struct {
	suite.Suite
}

func (test *MessageOrchestrationTest) SetupTest() {
}

func TestMessageOrchestration(t *testing.T) {
	suite.Run(t, new(MessageOrchestrationTest))
}

func (test *MessageOrchestrationTest) TestNewMessageOrchestration() {
	test.Run("creation", func() {
		orchestration := NewMessageOrchestration("test")
		test.NotNil(orchestration)
		test.Equal("test", orchestration.Name)
		test.NotNil(orchestration.Steps)
		test.Empty(orchestration.Steps)
		test.Nil(orchestration.FallbackStep)
	})
	test.Run("add step", func() {
		orchestration := NewMessageOrchestration("test orchestration")
		test.NotNil(orchestration)
		test.Equal("test orchestration", orchestration.Name)
		test.NotNil(orchestration.Steps)
		test.Empty(orchestration.Steps)
		test.Nil(orchestration.FallbackStep)

		m, err := NewMessage("module", "component", "method", DemoEntity{Key: "key", Value: 1})
		test.NoError(err)

		orchestration.AddStep("test step", []Message{m})
		test.Len(orchestration.Steps, 1)
		test.Equal("test step", orchestration.Steps[0].Name)
		test.Len(orchestration.Steps[0].Commands, 1)
		test.NotEmpty(orchestration.Steps[0].Commands[0].(*message).OrchestrationID)
		test.NotEmpty(orchestration.Steps[0].Commands[0].(*message).OrchestrationName)
		test.Equal("test step", *orchestration.Steps[0].Commands[0].(*message).OrchestrationStep)
		test.Equal(0, *orchestration.Steps[0].Commands[0].(*message).OrchestrationStepNumber)
		test.False(*orchestration.Steps[0].Commands[0].(*message).OrchestrationFallbackStep)
	})
	test.Run("set fallback", func() {
		orchestration := NewMessageOrchestration("test")
		test.NotNil(orchestration)
		test.Equal("test", orchestration.Name)
		test.NotNil(orchestration.Steps)
		test.Empty(orchestration.Steps)
		test.Nil(orchestration.FallbackStep)

		m, err := NewMessage("module", "component", "method", DemoEntity{Key: "key", Value: 1})
		test.NoError(err)

		orchestration.SettFallback([]Message{m})
		test.NotNil(orchestration.FallbackStep)
		test.Equal("fallback", orchestration.FallbackStep.Name)
		test.Len(orchestration.FallbackStep.Commands, 1)
		test.NotEmpty(orchestration.FallbackStep.Commands[0].(*message).OrchestrationID)
		test.NotEmpty(orchestration.FallbackStep.Commands[0].(*message).OrchestrationName)
		test.Equal("fallback", *orchestration.FallbackStep.Commands[0].(*message).OrchestrationStep)
	})
}
