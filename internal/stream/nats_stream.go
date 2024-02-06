package stream

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"
)

func NewJetstream() (nats.JetStreamContext, error) {
	nc, err := nats.Connect(environment.GetNATSURL())
	if err != nil {
		return nil, err
	}
	jt, err := nc.JetStream()
	if err != nil {
		return nil, err
	}
	_, err = jt.AddStream(&nats.StreamConfig{
		Name:     "foreverbull",
		Subjects: []string{"foreverbull.>"},
	})
	return jt, err
}

type NATSStream struct {
	module string

	jt   nats.JetStreamContext
	subs []*nats.Subscription

	deps *dependencyContainer

	repository repository
}

func NewNATSStream(jt nats.JetStreamContext, module string, dc DependencyContainer, db *pgxpool.Pool) (Stream, error) {
	cfg := &nats.ConsumerConfig{
		Name:       module,
		Durable:    module,
		MaxDeliver: 1,
		AckPolicy:  nats.AckExplicitPolicy,
	}
	_, err := jt.AddConsumer("foreverbull", cfg)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(context.Background(), table)
	if err != nil {
		return nil, err
	}
	return &NATSStream{module: module, jt: jt, deps: dc.(*dependencyContainer), repository: NewRepository(db)}, nil
}

func (ns *NATSStream) CreateMessage(ctx context.Context, msg Message) (Message, error) {
	m := msg.(*message)
	err := ns.repository.CreateMessage(ctx, m)
	if err != nil {
		return nil, err
	}
	return ns.repository.GetMessage(ctx, *m.ID)
}

func (ns *NATSStream) CommandSubscriber(component, method string, cb func(context.Context, Message) error) error {
	jtCb := func(m *nats.Msg) {
		// For now we just ack the message
		if err := m.Ack(); err != nil {
			log.Err(err).Msg("error acknowledging message")
		}
		go func(m *nats.Msg) {
			defer func() {
				if r := recover(); r != nil {
					log.Err(fmt.Errorf("panic: %v", r)).Msg("panic in command subscriber")
				}
			}()
			msg := &message{}
			err := json.Unmarshal(m.Data, &msg)
			if err != nil {
				log.Err(err).Msg("error unmarshalling message")
			}
			ctx := context.Background()
			msg, err = ns.repository.GetMessage(ctx, *msg.ID)
			if err != nil {
				log.Err(err).Msg("error getting message")
				return
			}
			if msg.StatusHistory == nil || len(msg.StatusHistory) == 0 {
				log.Debug().Msg("message has no status history")
				return
			}
			if msg.StatusHistory[0].Status != MessageStatusPublished {
				log.Debug().Msg("message has not been published")
				return
			}

			log := log.With().Str("id", *msg.ID).Logger()
			log.Info().Msg("received message")

			err = ns.repository.UpdateMessageStatus(ctx, *msg.ID, MessageStatusReceived, nil)
			if err != nil {
				log.Err(err).Msg("error updating message status")
				return
			}

			msg.dependencyContainer = ns.deps
			err = cb(ctx, msg)
			if err != nil {
				log.Err(err).Msg("error executing command")
			} else {
				log.Info().Msg("command executed")
			}
			err = ns.repository.UpdateMessageStatus(ctx, *msg.ID, MessageStatusComplete, err)
			if err != nil {
				log.Err(err).Msg("error updating message status")
				return
			}
			payload, err := json.Marshal(msg)
			if err != nil {
				log.Err(err).Msg("error marshalling message")
				return
			}
			_, err = ns.jt.Publish(fmt.Sprintf("foreverbull.%s.%s.%s.event", ns.module, component, method), payload)
			if err != nil {
				log.Err(err).Msg("error publishing event")
				return
			}
			log.Info().Msg("published event")
		}(m)
	}
	opts := []nats.SubOpt{
		nats.MaxDeliver(1),
		nats.Durable(fmt.Sprintf("foreverbull-%s-%s-%s", ns.module, component, method)),
	}
	switch environment.GetNATSDeliveryPolicy() {
	case "all":
		opts = append(opts, nats.DeliverAll())
	case "last":
		opts = append(opts, nats.DeliverLast())
	default:
		return fmt.Errorf("unknown delivery policy: %s", environment.GetNATSDeliveryPolicy())
	}

	sub, err := ns.jt.Subscribe(fmt.Sprintf("foreverbull.%s.%s.%s.command", ns.module, component, method), jtCb, opts...)
	if err != nil {
		return fmt.Errorf("error subscribing to jetstream: %w", err)
	}
	ns.subs = append(ns.subs, sub)
	return nil
}

func (ns *NATSStream) Unsubscribe() error {
	for _, sub := range ns.subs {
		if !sub.IsValid() {
			continue
		}
		err := sub.Drain()
		if err != nil {
			return err
		}
	}
	return nil
}

func (ns *NATSStream) Publish(ctx context.Context, msg Message) error {
	m := msg.(*message)
	if m.ID == nil {
		return fmt.Errorf("message id is nil")
	}
	topic := fmt.Sprintf("foreverbull.%s.%s.%s.command", m.Module, m.Component, m.Method)
	payload, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("error marshalling message: %w", err)
	}
	err = ns.repository.UpdateMessageStatus(ctx, *m.ID, MessageStatusPublished, err)
	if err != nil {
		return fmt.Errorf("error updating message status: %w", err)
	}

	_, err = ns.jt.Publish(topic, payload)
	if err != nil {
		return fmt.Errorf("error publishing message: %w", err)
	}
	return nil
}
