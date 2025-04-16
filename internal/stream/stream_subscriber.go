package stream

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/lhjnilsson/foreverbull/internal/stream/pb"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type NatsSubscriber struct {
	module string

	jt   nats.JetStreamContext
	subs []*nats.Subscription

	deps *dependencyContainer
}

func (ns *NatsSubscriber) Subscribe(component, method string, cb func(context.Context, Message) error) error {
	publishState := func(msg *message, state pb.State_State, err error) error {
		var s pb.State
		if err == nil {
			s = pb.State{State: state, OccurredAt: timestamppb.Now()}
		} else {
			errStr := err.Error()
			s = pb.State{State: state, Error: &errStr, OccurredAt: timestamppb.Now()}
		}
		payload, err := proto.Marshal(&s)
		if err != nil {
			return fmt.Errorf("error marshalling state: %w", err)
		}

		s2 := pb.State{}
		err = proto.Unmarshal(payload, &s2)
		if err != nil {
			return fmt.Errorf("error unmarshalling state: %w", err)
		}
		_, err = ns.jt.Publish(fmt.Sprintf(TOPIC_TASK_STATE, ns.module, msg.msg.Component, msg.msg.Method, msg.msg.GetId()), payload)
		if err != nil {
			return fmt.Errorf("error publishing state: %w", err)
		}
		return nil
	}

	jtCb := func(natsMsg *nats.Msg) {
		defer func() {
			if r := recover(); r != nil {
				fmt.Println(string(debug.Stack())) // nolint: forbidigo
				log.Err(fmt.Errorf("panic: %v", r)).Stack().Str("component", component).Str("method", method).Msg("panic in command subscriber")
			}
		}()

		msg := &message{
			msg:                 &pb.Message{},
			dependencyContainer: ns.deps,
		}
		err := proto.Unmarshal(natsMsg.Data, msg.msg)
		if err != nil {
			log.Err(err).Any("data", string(natsMsg.Data)).Msg("error unmarshalling message")
			return
		}
		err = publishState(msg, pb.State_STATE_RECEIVED, nil)
		if err != nil {
			log.Err(err).Msg("error publishing state")
			return
		}
		ctx := context.Background()
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		sub, err := ns.jt.Subscribe(fmt.Sprintf(TOPIC_TASK_ABORT, ns.module, msg.msg.Component, msg.msg.Method, msg.msg.GetId()), func(natsMsg *nats.Msg) {
			log.Info().Msg("aborting")
			cancel()
		})
		if err != nil {
			log.Err(err).Msg("error subscribing to abort")
			return
		}
		defer func() {
			if err := sub.Unsubscribe(); err != nil {
				log.Err(err).Msg("error unsubscribing from jetstream")
			}
		}()

		err = cb(ctx, msg)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				log.Info().Msg("command canceled")
				if err := publishState(msg, pb.State_STATE_CANCELED, nil); err != nil {
					log.Err(err).Msg("error publishing state")
				}
				return
			}
			log.Err(err).Msg("error executing command")
			if err := publishState(msg, pb.State_STATE_ERROR, err); err != nil {
				log.Err(err).Msg("error publishing state")
			}
			return
		}
		if err := publishState(msg, pb.State_STATE_COMPLETED, nil); err != nil {
			log.Err(err).Msg("error publishing state")
		}
		log.Info().Msg("command completed successfully")
		payload, err := proto.Marshal(msg.msg)
		if err != nil {
			log.Err(err).Msg("error marshalling message")
			return
		}

		_, err = ns.jt.Publish(fmt.Sprintf("foreverbull.%s.%s.%s.event", ns.module, component, method), payload)
		if err != nil {
			log.Err(err).Msg("error publishing event")
			return
		}
	}

	topic := fmt.Sprintf(TOPIC_TASK, ns.module, component, method)
	name := strings.Replace(topic, ".", "-", -1)

	consumerCfg := nats.ConsumerConfig{
		Name:           name,
		Durable:        name,
		AckPolicy:      nats.AckNonePolicy,
		DeliverSubject: name,
		FilterSubject:  topic,
	}
	_, err := ns.jt.AddConsumer("foreverbull", &consumerCfg)
	if err != nil {
		return fmt.Errorf("error creating consumer: %w", err)
	}

	opts := []nats.SubOpt{
		nats.Durable(name),
	}

	/* Unsure if this is needed
	deliverPolicy := environment.GetNATSDeliveryPolicy()
	switch deliverPolicy {
	case "all":
		fmt.Println("ALL")
		opts = append(opts, nats.DeliverAll())
	case "last":
		fmt.Println("LAST")
		opts = append(opts, nats.DeliverLast())
	default:
		return fmt.Errorf("unknown delivery policy: %s", environment.GetNATSDeliveryPolicy())
	}
	*/

	sub, err := ns.jt.Subscribe(fmt.Sprintf(TOPIC_TASK, ns.module, component, method), jtCb, opts...)
	if err != nil {
		return fmt.Errorf("error subscribing to jetstream: %w", err)
	}

	ns.subs = append(ns.subs, sub)
	return nil
}

func (ns *NatsSubscriber) Unsubscribe() error {
	for _, sub := range ns.subs {
		if !sub.IsValid() {
			continue
		}

		err := sub.Drain()
		if err != nil {
			return fmt.Errorf("error draining subscription: %w", err)
		}
	}

	return nil
}
