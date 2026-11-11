package stream

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/lhjnilsson/foreverbull/internal/stream/pb"
	"github.com/lhjnilsson/foreverbull/internal/usercopy"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type NatsSubscriber struct {
	module string

	jt   nats.JetStreamContext
	nc   *nats.Conn
	subs []*nats.Subscription

	deps *dependencyContainer
}

func (ns *NatsSubscriber) Subscribe(component, method string, cb func(context.Context, Message) error) error {
	// publishState sets Error only for err; notices ride in their own field
	// so a completed state never carries an error.
	publishState := func(msg *message, state pb.State_State, err error, notices ...string) error {
		s := pb.State{State: state, OccurredAt: timestamppb.Now(), Notices: notices}
		if err != nil {
			errStr := err.Error()
			s.Error = &errStr
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
		subject := fmt.Sprintf(TOPIC_TASK_STATE, ns.module, msg.msg.Component, msg.msg.Method, msg.msg.GetId())
		_, err = ns.jt.Publish(subject, payload)
		if err != nil {
			pubErr := fmt.Errorf("error publishing state %s: %w", state, err)
			if !refused(err) {
				// A lost ack, a deadline, or a dropped connection: the
				// server may have stored the state. A final state is read
				// back once; a state that does not end the task is left
				// unreported.
				return ns.unconfirmed(msg, state, subject, payload, &s2, pubErr)
			}
			// The stream refused the state, so nobody following the stream
			// sees it. Report it over core NATS so the task still ends.
			if reportErr := ns.reportUnrecorded(msg, &s2, err); reportErr != nil {
				return errors.Join(pubErr, reportErr)
			}
			return pubErr
		}
		return nil
	}

	jtCb := func(natsMsg *nats.Msg) {
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
		var unconfirmed *unconfirmedError
		switch {
		case errors.As(err, &unconfirmed):
			// RECEIVED may be stored. Stopping here would leave the waiter
			// on a task that never ends, so the command runs; the waiter
			// doesn't need RECEIVED to see the final state.
			log.Err(err).Str("id", msg.msg.GetId()).Msg("RECEIVED may not be stored; running the command")
		case err != nil:
			log.Err(err).Str("id", msg.msg.GetId()).Msg("error publishing state")
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
			// Without the abort subscription the command can't be cancelled,
			// so it isn't run. The sender is waiting on RECEIVED; tell it.
			err = Unreachable(fmt.Errorf("error subscribing to abort: %w", err))
			log.Err(err).Str("id", msg.msg.GetId()).Msg("error starting command")
			if pubErr := publishState(msg, pb.State_STATE_ERROR, stateError(err)); pubErr != nil {
				log.Err(pubErr).Str("id", msg.msg.GetId()).Msg("error publishing state")
			}
			return
		}
		defer func() {
			if err := sub.Unsubscribe(); err != nil {
				log.Err(err).Msg("error unsubscribing from jetstream")
			}
		}()
		defer func() {
			if r := recover(); r != nil {
				fmt.Println(string(debug.Stack())) // nolint: forbidigo
				log.Err(fmt.Errorf("panic: %v", r)).Stack().Str("component", component).Str("method", method).Msg("panic in command subscriber")
				if err := publishState(msg, pb.State_STATE_ERROR, fmt.Errorf("panic: %v", r)); err != nil {
					log.Err(err).Msg("error publishing state")
				}
			}
		}()
		// Progress goes over core NATS, not as a task state: it isn't kept
		// on the JetStream stream and gets no message_states row.
		progressTopic := fmt.Sprintf(TOPIC_TASK_PROGRESS, ns.module, msg.msg.Component, msg.msg.Method, msg.msg.GetId())
		ctx = WithProgress(ctx, func(text string) {
			if ns.nc == nil {
				return
			}
			if err := ns.nc.Publish(progressTopic, []byte(text)); err != nil {
				log.Err(err).Msg("error publishing progress")
			}
		})
		err = cb(ctx, msg)
		var notices []string
		var notice *Notice
		if errors.As(err, &notice) {
			// A notice is a completed command; its lines ride on the state.
			notices = notice.Lines
			err = nil
		}
		if err == nil {
			// The event goes out before COMPLETED, so a failed publish is
			// reported as ERROR rather than after the sender saw success.
			err = publishEvent(ns.jt, ns.module, component, method, msg.msg)
		}
		if err != nil {
			if errors.Is(err, context.Canceled) {
				log.Info().Msg("command canceled")
				if err := publishState(msg, pb.State_STATE_CANCELED, nil); err != nil {
					log.Err(err).Str("id", msg.msg.GetId()).Msg("error publishing state")
				}
				return
			}
			logged, stored := commandReport(err)
			log.Err(logged).Msg("error executing command")
			if err := publishState(msg, pb.State_STATE_ERROR, stored); err != nil {
				log.Err(err).Str("id", msg.msg.GetId()).Msg("error publishing state")
			}
			return
		}
		if err := publishState(msg, pb.State_STATE_COMPLETED, nil, notices...); err != nil {
			// The callback has no caller. A final state the read showed
			// was not stored is already reported, so the waiter ends.
			log.Err(err).Str("id", msg.msg.GetId()).Msg("error publishing state")
			return
		}
		log.Info().Msg("command completed successfully")
	}

	topic := fmt.Sprintf(TOPIC_TASK, ns.module, component, method)
	name := strings.ReplaceAll(topic, ".", "-")
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

// unconfirmedError is a state publish whose outcome is unknown: the
// server may have stored it. Nothing is reported for it over core NATS,
// because a report could contradict that state. A final state is not
// this error when a read shows it was not stored; that one is reported.
type unconfirmedError struct{ err error }

func (e *unconfirmedError) Error() string {
	return e.err.Error() + " (the stream may have stored it)"
}
func (e *unconfirmedError) Unwrap() error { return e.err }

// unrecordedState is the state a worker reports over core NATS when the
// stream refused state with refusal. A final ERROR or CANCELED keeps its
// state and cause. Any other state becomes ERROR: a task whose RECEIVED or
// COMPLETED wasn't stored can't be followed to its end on the stream, so it
// fails with a named cause. The cause is ErrStateNotRecorded for a full
// stream and ErrServicesUnreachable for any other refusal. The NATS error
// itself goes only to the log.
func unrecordedState(state *pb.State, refusal error) *pb.State {
	if state.GetState() == pb.State_STATE_ERROR || state.GetState() == pb.State_STATE_CANCELED {
		return state
	}
	cause := ErrServicesUnreachable.Error()
	if streamFull(refusal) {
		cause = ErrStateNotRecorded.Error()
	}
	return &pb.State{State: pb.State_STATE_ERROR, OccurredAt: timestamppb.Now(), Error: &cause}
}

// reportUnrecorded sends the unrecordedState of state and the stream's
// refusal over core NATS on
// TOPIC_TASK_UNRECORDED, where WaitForCompletion and the state listener
// read it. Core NATS doesn't go through the stream, so it works when the
// stream refuses publishes. The flush makes a broken connection an error
// here rather than a message lost in a buffer.
func (ns *NatsSubscriber) reportUnrecorded(msg *message, state *pb.State, refusal error) error {
	return ns.publishUnrecorded(msg, unrecordedState(state, refusal))
}

// reportLostFinal sends the lostFinalState of a final publish whose
// acknowledgement was ambiguous and whose payload is not the last message
// on its subject. There is no second publish of that state.
func (ns *NatsSubscriber) reportLostFinal(msg *message, state *pb.State) error {
	return ns.publishUnrecorded(msg, lostFinalState(state))
}

func (ns *NatsSubscriber) publishUnrecorded(msg *message, state *pb.State) error {
	topic := fmt.Sprintf(TOPIC_TASK_UNRECORDED, ns.module, msg.msg.Component, msg.msg.Method, msg.msg.GetId())
	return ns.publishCore(topic, state, "an unrecorded state")
}

// reportUnconfirmed tells the waiter that a final state's outcome could
// not be checked. It goes on TOPIC_TASK_UNCONFIRMED, which the state
// listener does not read, so message_states gains no row that could
// contradict a success the read could not see. There is no second publish
// of the state.
func (ns *NatsSubscriber) reportUnconfirmed(msg *message) error {
	cause := usercopy.TaskEndNotConfirmed
	state := &pb.State{State: pb.State_STATE_ERROR, OccurredAt: timestamppb.Now(), Error: &cause}
	topic := fmt.Sprintf(TOPIC_TASK_UNCONFIRMED, ns.module, msg.msg.Component, msg.msg.Method, msg.msg.GetId())
	return ns.publishCore(topic, state, "an unconfirmed outcome")
}

func (ns *NatsSubscriber) publishCore(topic string, state *pb.State, what string) error {
	if ns.nc == nil {
		return fmt.Errorf("error reporting %s: no core NATS connection", what)
	}
	payload, err := proto.Marshal(state)
	if err != nil {
		return fmt.Errorf("error marshalling %s: %w", what, err)
	}
	if err := ns.nc.Publish(topic, payload); err != nil {
		return fmt.Errorf("error reporting %s: %w", what, err)
	}
	if err := ns.nc.Flush(); err != nil {
		return fmt.Errorf("error reporting %s: %w", what, err)
	}
	return nil
}

// endsTask reports whether state is one the waiter stops on.
func endsTask(state pb.State_State) bool {
	switch state {
	case pb.State_STATE_COMPLETED, pb.State_STATE_ERROR, pb.State_STATE_CANCELED:
		return true
	case pb.State_STATE_UNSPECIFIED, pb.State_STATE_CREATED,
		pb.State_STATE_PUBLISHED, pb.State_STATE_RECEIVED:
		return false
	default:
		return false
	}
}

// unconfirmed handles a publish whose acknowledgement doesn't settle
// whether the server stored it. A final state is read once. When that
// message is the payload just sent, the waiter will see it and nothing
// that could contradict it is reported. When it is absent, it is reported
// on the unrecorded subject so the waiter ends and message_states records
// it. When the read itself fails, the waiter is told on a subject the
// state listener does not read, so no error row is written. A state that
// doesn't end the task is not read: RECEIVED may be missing and the
// command still has to run.
func (ns *NatsSubscriber) unconfirmed(msg *message, state pb.State_State, subject string, payload []byte, sent *pb.State, pubErr error) error {
	if endsTask(state) {
		stored, err := ns.payloadStored(subject, payload)
		if err != nil {
			if reportErr := ns.reportUnconfirmed(msg); reportErr != nil {
				return &unconfirmedError{err: errors.Join(pubErr, err, reportErr)}
			}
			return &unconfirmedError{err: errors.Join(pubErr, err)}
		}
		if stored {
			log.Err(pubErr).Str("id", msg.msg.GetId()).Msg("final state is stored")
			return nil
		}
		if err := ns.reportLostFinal(msg, sent); err != nil {
			return errors.Join(pubErr, err)
		}
		return pubErr
	}
	return &unconfirmedError{err: pubErr}
}

// payloadStored reports whether the last message on subject is payload.
// A subject with no message does not hold it. Any other read error is
// returned so the caller does not report a failure over a state it
// could not see.
func (ns *NatsSubscriber) payloadStored(subject string, payload []byte) (bool, error) {
	msg, err := ns.jt.GetLastMsg(StreamName, subject)
	if errors.Is(err, nats.ErrMsgNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("error reading the stored task state: %w", err)
	}
	return bytes.Equal(msg.Data, payload), nil
}

// lostFinalState is the state reported when a final publish came back
// without a definite refusal and the subject does not hold that payload.
// ERROR and CANCELED keep their state and cause. COMPLETED becomes ERROR:
// a success that was not stored can't be reported as finished. The cause
// is ErrTaskFinishedNotSaved, not a full queue and not an unreachable
// service. The NATS error stays in the log.
func lostFinalState(state *pb.State) *pb.State {
	if state.GetState() == pb.State_STATE_ERROR || state.GetState() == pb.State_STATE_CANCELED {
		return state
	}
	cause := usercopy.TaskFinishedNotSaved
	return &pb.State{State: pb.State_STATE_ERROR, OccurredAt: timestamppb.Now(), Error: &cause}
}

type eventPublisher interface {
	Publish(subj string, data []byte, opts ...nats.PubOpt) (*nats.PubAck, error)
}

// publishEvent announces a completed command on its event subject. Only a
// definite refusal (refused) fails the command: a full stream with
// ErrStateNotRecorded, any other with ErrServicesUnreachable.
func publishEvent(js eventPublisher, module, component, method string, msg *pb.Message) error {
	payload, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("error marshalling event: %w", err)
	}
	subject := fmt.Sprintf("foreverbull.%s.%s.%s.event", module, component, method)
	if _, err := js.Publish(subject, payload); err != nil {
		switch {
		case streamFull(err):
			return notRecorded(fmt.Errorf("error publishing event: %w", err))
		case refused(err):
			return Unreachable(fmt.Errorf("error publishing event: %w", err))
		default:
			// A lost ack, a deadline, or a dropped connection: the stream
			// may have stored the event, so the command isn't failed for
			// it. Nothing in the broker subscribes to events, so a lost
			// one leaves no task waiting.
			log.Err(err).Str("subject", subject).Str("id", msg.GetId()).
				Msg("event may not be stored; completing the command")
		}
	}
	return nil
}
