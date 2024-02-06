package stream

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"
	"go.uber.org/fx"
)

type PendingOrchestration struct {
	orchestrations []*MessageOrchestration
}

func (po *PendingOrchestration) Add(orchestration *MessageOrchestration) {
	po.orchestrations = append(po.orchestrations, orchestration)
}

func (po *PendingOrchestration) Get() []*MessageOrchestration {
	return po.orchestrations
}

func (po *PendingOrchestration) Contains(name string) bool {
	for _, orchestration := range po.orchestrations {
		if orchestration.Name == name {
			return true
		}
	}
	return false
}

func (ns *NATSStream) CreateOrchestration(ctx context.Context, orchestration *MessageOrchestration) error {
	for _, step := range orchestration.Steps {
		for _, cmd := range step.Commands {
			msg, ok := cmd.(*message)
			if msg.OrchestrationID == nil {
				return fmt.Errorf("orchestration id is nil")
			}
			if msg.OrchestrationStep == nil {
				return fmt.Errorf("orchestration step is nil")
			}

			if !ok {
				return fmt.Errorf("command is not a message")
			}
			err := ns.repository.CreateMessage(ctx, msg)
			if err != nil {
				return err
			}
		}
	}
	if orchestration.FallbackStep == nil {
		return fmt.Errorf("orchestration must have fallback step")
	}
	for _, cmd := range orchestration.FallbackStep.Commands {
		msg, ok := cmd.(*message)
		if msg.OrchestrationID == nil {
			return fmt.Errorf("orchestration id is nil")
		}
		if msg.OrchestrationStep == nil {
			return fmt.Errorf("orchestration step is nil")
		}
		if !ok {
			return fmt.Errorf("command is not a message")
		}
		err := ns.repository.CreateMessage(ctx, msg)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ns *NATSStream) RunOrchestration(ctx context.Context, orchestrationID string) error {
	isRunning, err := ns.repository.OrchestrationIsRunning(ctx, orchestrationID)
	if err != nil {
		return err
	}
	if isRunning {
		return fmt.Errorf("orchestration is already running")
	}
	commands, err := ns.repository.GetLatestUnpublishedOrchestrationStepCommands(ctx, orchestrationID)
	if err != nil {
		return fmt.Errorf("error getting latest unpublished orchestration step commands: %w", err)
	}
	for _, cmd := range *commands {
		err = ns.Publish(ctx, &cmd)
		if err != nil {
			return fmt.Errorf("error publishing command: %w", err)
		}
	}
	return nil
}

func NewOrchestrationRunner(stream *NATSStream) (*OrchestrationRunner, error) {
	return &OrchestrationRunner{
		stream: stream,
	}, nil
}

type OrchestrationRunner struct {
	stream *NATSStream

	sub *nats.Subscription
}

func (or *OrchestrationRunner) msgHandler(natsMsg *nats.Msg) {
	msg := &message{}
	err := json.Unmarshal(natsMsg.Data, &msg)
	if err != nil {
		log.Err(err).Msg("error unmarshalling message")
		return
	}
	ctx := context.Background()
	msg, err = or.stream.repository.GetMessage(ctx, *msg.ID)
	if err != nil {
		log.Err(err).Msg("error getting message")
		return
	}
	log := log.With().Str("id", *msg.ID).Logger()
	log.Info().Msg("received message")

	if msg.OrchestrationID == nil {
		log.Debug().Msg("message does not have orchestration id")
		return
	}

	complete, err := or.stream.repository.OrchestrationIsComplete(ctx, *msg.OrchestrationID)
	if err != nil {
		log.Err(err).Msg("error checking if orchestration is complete")
		return
	}
	if complete {
		err = or.stream.repository.MarkAllCreatedAsCanceled(ctx, *msg.OrchestrationID)
		if err != nil {
			log.Err(err).Msg("error marking all created as canceled")
		}
		return
	}
	complete, err = or.stream.repository.OrchestrationStepIsComplete(ctx, *msg.OrchestrationID, *msg.OrchestrationStep)
	if err != nil {
		log.Err(err).Msg("error checking if orchestration step is complete")
		return
	}
	if !complete {
		log.Debug().Msg("orchestration step is not complete")
		return
	}
	var commands *[]message

	failing, err := or.stream.repository.OrchestrationHasError(ctx, *msg.OrchestrationID)
	if err != nil {
		log.Err(err).Msg("error checking if orchestration has error")
		return
	}
	if failing {
		commands, err = or.stream.repository.GetOrchestrationFallbackCommands(ctx, *msg.OrchestrationID)
		if err != nil {
			log.Err(err).Msg("error getting orchestration fallback commands")
			return
		}
		defer func() {
			err = or.stream.repository.MarkAllCreatedAsCanceled(ctx, *msg.OrchestrationID)
			if err != nil {
				log.Err(err).Msg("error marking all created as canceled")
			}
		}()
	} else {
		commands, err = or.stream.repository.GetLatestUnpublishedOrchestrationStepCommands(ctx, *msg.OrchestrationID)
		if err != nil {
			log.Err(err).Msg("error getting latest unpublished orchestration step commands")
			return
		}
	}
	for _, cmd := range *commands {
		err = or.stream.Publish(ctx, &cmd)
		if err != nil {
			log.Err(err).Msg("error publishing command")
			return
		}
	}
}
func (or *OrchestrationRunner) Start() error {
	var err error
	opts := []nats.SubOpt{
		nats.MaxDeliver(1),
		nats.Durable("foreverbull-orchestration-event"),
	}
	switch environment.GetNATSDeliveryPolicy() {
	case "all":
		opts = append(opts, nats.DeliverAll())
	case "last":
		opts = append(opts, nats.DeliverLast())
	default:
		return fmt.Errorf("unknown delivery policy: %s", environment.GetNATSDeliveryPolicy())
	}

	or.sub, err = or.stream.jt.Subscribe("foreverbull.*.*.*.event", or.msgHandler, opts...)
	if err != nil {
		return fmt.Errorf("error subscribing to jetstream for orchestration: %w", err)
	}
	return nil
}

func (or *OrchestrationRunner) Stop() error {
	return or.sub.Unsubscribe()
}

type OrchestrationStream NATSStream

var OrchestrationLifecycle = fx.Options(
	fx.Provide(
		func(jt nats.JetStreamContext, db *pgxpool.Pool) (*OrchestrationRunner, error) {
			cfg := nats.ConsumerConfig{
				Name:       "orchestration-event",
				Durable:    "orchestration-event",
				MaxDeliver: 1,
			}
			_, err := jt.AddConsumer("foreverbull", &cfg)
			if err != nil {
				return nil, fmt.Errorf("error adding consumer for orchestration: %w", err)
			}
			dc := NewDependencyContainer().(*dependencyContainer)
			stream := &NATSStream{module: "orchestration", jt: jt, repository: NewRepository(db), deps: dc}
			return NewOrchestrationRunner(stream)
		},
	),
	fx.Invoke(
		func(lc fx.Lifecycle, or *OrchestrationRunner) error {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					return or.Start()
				},
				OnStop: func(ctx context.Context) error {
					return or.Stop()
				},
			})
			return nil
		}),
)
