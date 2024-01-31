package stream

import (
	"context"
)

type Handler interface {
	Process(ctx context.Context, message Message) error
}

type Stream interface {
	CreateMessage(ctx context.Context, message Message) (Message, error)
	CreateOrchestration(ctx context.Context, orchestration *MessageOrchestration) error
	Unsubscribe() error
	Publish(ctx context.Context, message Message) error
	CommandSubscriber(component, method string, cb func(context.Context, Message) error) error
	RunOrchestration(ctx context.Context, orchestrationID string) error
}
