package stream

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

func NewJetstream(uri string) (nats.JetStreamContext, error) {
	nc, err := nats.Connect(uri)
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
	log  *zap.Logger
	subs []*nats.Subscription

	deps *dependencyContainer

	repository repository
}

func NewNATSStream(jt nats.JetStreamContext, module string, log *zap.Logger, dc DependencyContainer, db *pgxpool.Pool) (Stream, error) {
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
	return &NATSStream{module: module, jt: jt, log: log, deps: dc.(*dependencyContainer), repository: NewRepository(db)}, nil
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

			ns.log.Error("error acknowledging message", zap.Error(err))
		}
		go func(m *nats.Msg) {
			defer func() {
				if r := recover(); r != nil {
					ns.log.Error("recovered from panic", zap.Any("error", r))
				}
			}()
			msg := &message{}
			err := json.Unmarshal(m.Data, &msg)
			if err != nil {
				ns.log.Error("error unmarshalling message", zap.Error(err))
			}
			ctx := context.Background()
			msg, err = ns.repository.GetMessage(ctx, *msg.ID)
			if err != nil {
				ns.log.Error("error getting message", zap.Error(err))
				return
			}
			if msg.StatusHistory == nil || len(msg.StatusHistory) == 0 {
				ns.log.Info("message has no status history")
				return
			}
			if msg.StatusHistory[0].Status != MessageStatusPublished {
				ns.log.Info("message is not published", zap.Any("status", msg.StatusHistory[0].Status))
				return
			}

			err = ns.repository.UpdateMessageStatus(ctx, *msg.ID, MessageStatusReceived, nil)
			if err != nil {
				ns.log.Error("error updating message status", zap.Error(err))
			}

			ns.log.Info("received command", zap.Any("module", msg.Module), zap.Any("component", msg.Component), zap.Any("method", msg.Method))
			msg.dependencyContainer = ns.deps
			err = cb(ctx, msg)
			if err != nil {
				ns.log.Error("error executing command", zap.Any("module", msg.Module), zap.Any("component", msg.Component), zap.Any("method", msg.Method), zap.Error(err))
			} else {
				ns.log.Info("command completed", zap.Any("module", msg.Module), zap.Any("component", msg.Component), zap.Any("method", msg.Method), zap.Any("error", err))
			}
			err = ns.repository.UpdateMessageStatus(ctx, *msg.ID, MessageStatusComplete, err)
			if err != nil {
				ns.log.Error("error updating message status", zap.Error(err))
				return
			}
			payload, err := json.Marshal(msg)
			if err != nil {
				ns.log.Error("error marshalling message", zap.Error(err))
				return
			}
			ns.jt.Publish(fmt.Sprintf("foreverbull.%s.%s.%s.event", ns.module, component, method), payload)
			ns.log.Info("published event", zap.Any("module", ns.module), zap.Any("component", component), zap.Any("method", method))
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
	ns.log.Info("published command", zap.Any("module", m.Module), zap.Any("component", m.Component), zap.Any("method", m.Method), zap.Any("orchestrationID", m.OrchestrationID), zap.Any("orchestrationStep", m.OrchestrationStep))
	return nil
}
