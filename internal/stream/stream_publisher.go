package stream

import (
	"context"
	"errors"
	"fmt"

	"github.com/lhjnilsson/foreverbull/internal/stream/pb"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type PublishOpt func(ctx context.Context, nn *NATSStream, msg *message) error

func WaitForCompletion() PublishOpt {
	return func(ctx context.Context, nn *NATSStream, msg *message) error {
		sub, err := nn.jt.SubscribeSync(msg.TaskStateTopic())
		if err != nil {
			return fmt.Errorf("error subscribing to jetstream: %w", err)
		}
		defer func() {
			if err := sub.Unsubscribe(); err != nil {
				log.Err(err).Msg("error unsubscribing from jetstream")
			}
		}()
		for {
			stateMsg, err := sub.NextMsgWithContext(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					_, err := nn.jt.Publish(fmt.Sprintf(TOPIC_TASK_ABORT, msg.msg.Module, msg.msg.Component, msg.msg.Method, msg.msg.GetId()), nil)
					if err != nil {
						return fmt.Errorf("error publishing abort: %w", err)
					}
					return context.Canceled
				}
				return fmt.Errorf("error getting message: %w", err)
			}
			state := pb.State{}
			err = proto.Unmarshal(stateMsg.Data, &state)
			if err != nil {
				return fmt.Errorf("error unmarshalling message: %w", err)
			}
			if err := stateMsg.Ack(); err != nil {
				return fmt.Errorf("error acking message: %w", err)
			}

			if state.State == pb.State_STATE_COMPLETED {
				return nil
			}

			if state.State == pb.State_STATE_ERROR {
				if state.GetError() == "" {
					return errors.New("command failed")
				}
				return errors.New(state.GetError())
			}

			if state.State == pb.State_STATE_CANCELED {
				return fmt.Errorf("canceled")
			}
		}
	}
}

func (nn *NATSStream) Publish(ctx context.Context, msg Message, opts ...PublishOpt) error {
	m, isMsg := msg.(*message)
	if !isMsg {
		return fmt.Errorf("invalid message")
	}
	payload, err := proto.Marshal(m.msg)
	if err != nil {
		return fmt.Errorf("error marshalling message: %w", err)
	}
	state := pb.State{State: pb.State_STATE_PUBLISHED, OccurredAt: timestamppb.Now()}
	statePayload, err := proto.Marshal(&state)
	if err != nil {
		return fmt.Errorf("error marshalling state: %w", err)
	}
	_, err = nn.jt.Publish(msg.TaskTopic(), payload)
	if err != nil {
		return fmt.Errorf("error publishing message: %w", err)
	}

	_, err = nn.jt.Publish(msg.TaskStateTopic(), statePayload)
	if err != nil {
		return fmt.Errorf("error publishing state: %w", err)
	}

	grp, _ := errgroup.WithContext(ctx)
	for _, opt := range opts {
		grp.Go(func() error {
			return opt(ctx, nn, msg.(*message))
		})
	}
	if err := grp.Wait(); err != nil {
		return err
	}
	return nil
}
