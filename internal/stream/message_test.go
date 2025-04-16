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
		test.Require().NoError(err)
		test.NotNil(msg)

		parsed, ok := msg.(*message)
		test.True(ok)
		test.Nil(parsed.msg.Id)
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

type MessageOrchestrationTest struct {
	suite.Suite
}

func (test *MessageOrchestrationTest) SetupTest() {
}
