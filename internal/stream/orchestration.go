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

type OrchestrationOutput struct {
	orchestrations []*MessageOrchestration
}

func (po *OrchestrationOutput) Add(orchestration *MessageOrchestration) {
	po.orchestrations = append(po.orchestrations, orchestration)
}

func (po *OrchestrationOutput) Get() []*MessageOrchestration {
	return po.orchestrations
}

func (po *OrchestrationOutput) Contains(name string) bool {
	for _, orchestration := range po.orchestrations {
		if orchestration.Name == name {
			return true
		}
	}

	return false
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

	if msg.OrchestrationID == nil {
		log.Debug().Msg("event does not have orchestration id")
		return
	}

	log := log.With().Str("id", *msg.ID).Str("Orchestration", *msg.OrchestrationName).Str("OrchestrationID", *msg.OrchestrationID).Str("OrchestrationStep", *msg.OrchestrationStep).Logger()
	log.Info().Msg("received event")

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

	if msg.OrchestrationStepNumber == nil {
		// Could be a fallback step
		log.Debug().Msg("event does not have orchestration step number")
		return
	}

	commands, err := or.stream.repository.GetNextOrchestrationCommands(ctx, *msg.OrchestrationID, *msg.OrchestrationStepNumber)
	if err != nil {
		log.Err(err).Msg("error getting next orchestration commands")
		return
	}

	if commands == nil {
		log.Debug().Msg("no commands to run")
		return
	}

	if len(*commands) > 0 && (*commands)[0].OrchestrationFallbackStep != nil && *(*commands)[0].OrchestrationFallbackStep {
		log.Debug().Msg("orchestration is failing")

		defer func() {
			err = or.stream.repository.MarkAllCreatedAsCanceled(ctx, *msg.OrchestrationID)
			if err != nil {
				log.Err(err).Msg("error marking all created as canceled")
			}
		}()
	}

	for _, cmd := range *commands {
		log.Info().Str("CmdID", *cmd.ID).Str("CmdComponent", cmd.Component).Str("CmdMethod", cmd.Method).Msg("publishing command")

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
	deliverPolicy := environment.GetNATSDeliveryPolicy()

	switch deliverPolicy {
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
	if err := or.sub.Unsubscribe(); err != nil {
		return fmt.Errorf("error unsubscribing from jetstream for orchestration: %w", err)
	}

	return nil
}

var OrchestrationLifecycle = fx.Options( //nolint: gochecknoglobals
	fx.Provide(
		func(jetstream nats.JetStreamContext, pool *pgxpool.Pool) (*OrchestrationRunner, error) {
			cfg := nats.ConsumerConfig{
				Name:       "orchestration-event",
				Durable:    "orchestration-event",
				MaxDeliver: 1,
			}
			_, err := jetstream.AddConsumer("foreverbull", &cfg)
			if err != nil {
				return nil, fmt.Errorf("error adding consumer for orchestration: %w", err)
			}
			dc := NewDependencyContainer().(*dependencyContainer)
			stream := &NATSStream{module: "orchestration", jt: jetstream, repository: NewRepository(pool), deps: dc}
			return NewOrchestrationRunner(stream)
		},
	),
	fx.Invoke(
		func(lc fx.Lifecycle, orchestration *OrchestrationRunner) error {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					return orchestration.Start()
				},
				OnStop: func(ctx context.Context) error {
					return orchestration.Stop()
				},
			})
			return nil
		}),
)
