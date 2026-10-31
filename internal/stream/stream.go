package stream

import (
	"context"
	"fmt"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/stream/pb"
	"github.com/nats-io/nats.go"
)

const (
	TOPIC_TASK       = "foreverbull.%s.%s.%s"
	TOPIC_TASK_ABORT = "foreverbull.%s.%s.%s.%s.abort"
	TOPIC_TASK_STATE = "foreverbull.%s.%s.%s.%s.state"
	// TOPIC_TASK_PROGRESS is core NATS, outside the foreverbull stream's
	// foreverbull.> subjects: progress is not stored on JetStream and gets
	// no message_states row. A waiter that isn't listening misses it.
	TOPIC_TASK_PROGRESS = "foreverbull-progress.%s.%s.%s.%s"
)

type Stream interface {
	Publish(ctx context.Context, message Message, opts ...PublishOpt) error
	RunOrchestration(ctx context.Context, orchestration *pb.Orchestration) <-chan *pb.Orchestration_Step
	NewSubscriber(module string, dc DependencyContainer) (Subscriber, error)
}

type Subscriber interface {
	Subscribe(component, method string, cb func(context.Context, Message) error) error
	Unsubscribe() error
}

type NATSStream struct {
	jt nats.JetStreamContext
	// nc carries progress over core NATS.
	nc *nats.Conn
}

// NATSConnOptions sends the broker user when NATS_USER is set.
// An empty user stays anonymous so unit tests and a local server keep working.
func NATSConnOptions() []nats.Option {
	user := environment.GetNATSUser()
	if user == "" {
		return nil
	}
	return []nats.Option{nats.UserInfo(user, environment.GetNATSPassword())}
}

func NewNATSStream() (Stream, error) {
	natsConnect, err := nats.Connect(environment.GetNATSURL(), NATSConnOptions()...)
	if err != nil {
		return nil, fmt.Errorf("error connecting to nats: %w", err)
	}

	jetstream, err := natsConnect.JetStream()
	if err != nil {
		return nil, fmt.Errorf("error connecting to jetstream: %w", err)
	}

	_, err = jetstream.AddStream(&nats.StreamConfig{
		Name:     "foreverbull",
		Subjects: []string{"foreverbull.>"},
	})
	if err != nil {
		return nil, fmt.Errorf("error creating stream: %w", err)
	}

	return &NATSStream{
		jt: jetstream,
		nc: natsConnect,
	}, nil
}

func (nn *NATSStream) NewSubscriber(module string, dc DependencyContainer) (Subscriber, error) {
	return &NatsSubscriber{
		module: module,
		jt:     nn.jt,
		nc:     nn.nc,
		deps:   dc.(*dependencyContainer),
	}, nil
}
