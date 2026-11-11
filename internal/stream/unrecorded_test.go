package stream

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/stream/pb"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/internal/usercopy"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// errStreamFull is what the server answers a publish to a full stream with
// DiscardNew (checked against a real stream in
// TestStreamFullMatchesAFullStreamsRefusal).
var errStreamFull = &nats.APIError{Code: 503, ErrorCode: 10077, Description: "maximum bytes exceeded"}

// refusingJetStream is a JetStream whose stream refuses one task state,
// every event when refuseEvents is set, every abort when refuseAborts is set,
// or every task when refuseTasks is set, with err. With storeFirst the
// message is stored before err comes back, as with a lost ack.
type refusingJetStream struct {
	nats.JetStreamContext
	refuse       pb.State_State
	refuseEvents bool
	refuseAborts bool
	refuseTasks  bool
	storeFirst   bool
	err          error
	// readErr, when set, is what GetLastMsg returns. The publish error is
	// separate: a closed connection on the read can follow a timeout on
	// the publish, and the core connection used to tell the waiter stays up.
	readErr error
}

func (r refusingJetStream) Publish(subj string, data []byte, opts ...nats.PubOpt) (*nats.PubAck, error) {
	refusal := r.err
	if refusal == nil {
		refusal = errStreamFull
	}
	refuse := func() (*nats.PubAck, error) {
		if r.storeFirst {
			if _, err := r.JetStreamContext.Publish(subj, data, opts...); err != nil {
				return nil, err
			}
		}
		return nil, refusal
	}
	switch {
	case r.refuseEvents && strings.HasSuffix(subj, ".event"):
		return refuse()
	case r.refuseAborts && strings.HasSuffix(subj, ".abort"):
		return refuse()
	case r.refuseTasks && strings.Count(subj, ".") == 3:
		return refuse()
	}
	state := &pb.State{}
	if _, err := stateMessageID(subj); err == nil && proto.Unmarshal(data, state) == nil && state.GetState() == r.refuse {
		return refuse()
	}
	return r.JetStreamContext.Publish(subj, data, opts...)
}

func (r refusingJetStream) GetLastMsg(name, subject string, opts ...nats.JSOpt) (*nats.RawStreamMsg, error) {
	if r.readErr != nil {
		return nil, r.readErr
	}
	return r.JetStreamContext.GetLastMsg(name, subject, opts...)
}

func withJetStream(r refusingJetStream, js nats.JetStreamContext) refusingJetStream {
	r.JetStreamContext = js
	return r
}

type unrecordedSetup struct {
	waiter *NATSStream
	db     *pgxpool.Pool
}

// setupUnrecorded starts a worker whose stream refuses state, and a state
// listener for unrecorded states. The waiter is a normal NATSStream.
func setupUnrecorded(t *testing.T, refusing refusingJetStream, method string, cb func(context.Context, Message) error) unrecordedSetup {
	t.Helper()
	test_helper.SetupEnvironment(t, &test_helper.Containers{NATS: true, Postgres: true})

	db, err := pgxpool.New(context.Background(), environment.GetPostgresURL())
	require.NoError(t, err)
	t.Cleanup(db.Close)
	require.NoError(t, RecreateTables(context.Background(), db))

	s, err := NewNATSStream()
	require.NoError(t, err)
	waiter := s.(*NATSStream)
	t.Cleanup(waiter.nc.Close)

	worker, err := NewNATSStream()
	require.NoError(t, err)
	workerStream := worker.(*NATSStream)
	t.Cleanup(workerStream.nc.Close)
	sub := &NatsSubscriber{
		module: "unrecorded",
		jt:     withJetStream(refusing, workerStream.jt),
		nc:     workerStream.nc,
		deps:   NewDependencyContainer().(*dependencyContainer),
	}
	require.NoError(t, sub.Subscribe("refuse", method, cb))
	t.Cleanup(func() { require.NoError(t, sub.Unsubscribe()) })

	listenerConn, err := nats.Connect(environment.GetNATSURL())
	require.NoError(t, err)
	t.Cleanup(listenerConn.Close)
	listener := &stateSubscriber{repository: NewRepository(db), nc: listenerConn}
	require.NoError(t, listener.startUnrecorded())
	t.Cleanup(func() { require.NoError(t, listener.unrecordedSub.Unsubscribe()) })

	return unrecordedSetup{waiter: waiter, db: db}
}

// publishAndWait publishes method's task and waits for it, failing the test
// when the waiter doesn't return well inside the bound.
func (u unrecordedSetup) publishAndWait(t *testing.T, method string) (string, error) {
	t.Helper()
	msg, err := NewMessage("unrecorded", "refuse", method, nil)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err = u.waiter.Publish(ctx, msg, WaitForCompletion())
	require.NoError(t, ctx.Err(), "the waiter must return, not wait for a state the stream refused")
	return msg.GetMsg().GetId(), err
}

// requireRecordedCanceled waits for the listener to store the task's
// CANCELED row, which carries no cause.
func (u unrecordedSetup) requireRecordedCanceled(t *testing.T, id string) {
	t.Helper()
	require.Eventually(t, func() bool {
		var n int
		err := u.db.QueryRow(context.Background(),
			`SELECT count(*) FROM message_states WHERE message_id=$1 AND state=$2 AND error IS NULL`,
			id, int32(pb.State_STATE_CANCELED)).Scan(&n)
		return err == nil && n == 1
	}, 10*time.Second, 50*time.Millisecond, "message_states must record the cancellation")
}

// requireRecordedError waits for the listener to store the task's ERROR
// row with cause.
func (u unrecordedSetup) requireRecordedError(t *testing.T, id, cause string) {
	t.Helper()
	require.Eventually(t, func() bool {
		var n int
		err := u.db.QueryRow(context.Background(),
			`SELECT count(*) FROM message_states WHERE message_id=$1 AND state=$2 AND error=$3`,
			id, int32(pb.State_STATE_ERROR), cause).Scan(&n)
		return err == nil && n == 1
	}, 10*time.Second, 50*time.Millisecond, "message_states must name how the task ended")
}

func TestARefusedCompletedStateFailsTheTask(t *testing.T) {
	u := setupUnrecorded(t, refusingJetStream{refuse: pb.State_STATE_COMPLETED}, "completed",
		func(context.Context, Message) error { return nil })

	id, err := u.publishAndWait(t, "completed")
	require.ErrorIs(t, err, ErrStateNotRecorded)
	require.EqualError(t, err, usercopy.TaskStateNotRecorded)
	u.requireRecordedError(t, id, usercopy.TaskStateNotRecorded)
}

func TestARefusedReceivedStateFailsTheTaskWithoutRunningIt(t *testing.T) {
	var ran atomic.Bool
	u := setupUnrecorded(t, refusingJetStream{refuse: pb.State_STATE_RECEIVED}, "received",
		func(context.Context, Message) error { ran.Store(true); return nil })

	id, err := u.publishAndWait(t, "received")
	require.ErrorIs(t, err, ErrStateNotRecorded)
	require.False(t, ran.Load(), "a task whose RECEIVED wasn't stored must not run")
	u.requireRecordedError(t, id, usercopy.TaskStateNotRecorded)
}

func TestARefusedErrorStateKeepsItsCause(t *testing.T) {
	u := setupUnrecorded(t, refusingJetStream{refuse: pb.State_STATE_ERROR}, "error",
		func(context.Context, Message) error { return errors.New("the provider has no prices for XYZ") })

	id, err := u.publishAndWait(t, "error")
	require.EqualError(t, err, "the provider has no prices for XYZ")
	u.requireRecordedError(t, id, "the provider has no prices for XYZ")
}

func TestARefusedEventFailsTheTaskWithTheNamedCause(t *testing.T) {
	u := setupUnrecorded(t, refusingJetStream{refuseEvents: true}, "event",
		func(context.Context, Message) error { return nil })

	_, err := u.publishAndWait(t, "event")
	require.ErrorIs(t, err, ErrStateNotRecorded)
	require.EqualError(t, err, usercopy.TaskStateNotRecorded)
}

func TestAStateRefusedForAnotherReasonNamesTheUnreachableServices(t *testing.T) {
	u := setupUnrecorded(t, refusingJetStream{refuse: pb.State_STATE_COMPLETED, err: nats.ErrNoStreamResponse}, "noresponse",
		func(context.Context, Message) error { return nil })

	id, err := u.publishAndWait(t, "noresponse")
	require.ErrorIs(t, err, ErrServicesUnreachable)
	require.NotContains(t, err.Error(), "nats:")
	u.requireRecordedError(t, id, usercopy.ServicesUnreachable)
}

func TestAPublishTheStreamRefusesNamesTheFullQueueAndSendsNoTask(t *testing.T) {
	var ran atomic.Bool
	u := setupUnrecorded(t, refusingJetStream{}, "publisher",
		func(context.Context, Message) error { ran.Store(true); return nil })
	u.waiter.jt = refusingJetStream{JetStreamContext: u.waiter.jt, refuse: pb.State_STATE_PUBLISHED}

	id, err := u.publishAndWait(t, "publisher")
	require.ErrorIs(t, err, ErrStateNotRecorded)
	require.EqualError(t, err, usercopy.TaskStateNotRecorded, "the NATS error goes to the log only")
	var apiErr *nats.APIError
	require.ErrorAs(t, err, &apiErr, "the NATS error stays in the chain for the log")
	u.requireRecordedError(t, id, usercopy.TaskStateNotRecorded)
	require.False(t, ran.Load(), "a task whose PUBLISHED was refused must not be sent")
}

func TestAPublishedThatMayBeStoredStillFailsTheUnsentTask(t *testing.T) {
	var ran atomic.Bool
	u := setupUnrecorded(t, refusingJetStream{}, "publishedtimeout",
		func(context.Context, Message) error { ran.Store(true); return nil })
	u.waiter.jt = refusingJetStream{JetStreamContext: u.waiter.jt, refuse: pb.State_STATE_PUBLISHED, storeFirst: true, err: nats.ErrTimeout}

	id, err := u.publishAndWait(t, "publishedtimeout")
	require.ErrorIs(t, err, nats.ErrTimeout)
	u.requireRecordedError(t, id, usercopy.ServicesUnreachable)
	require.False(t, ran.Load(), "the task is never sent after a failed PUBLISHED")
}

func TestAnEventThatMayBeStoredCompletesTheTask(t *testing.T) {
	u := setupUnrecorded(t, refusingJetStream{refuseEvents: true, storeFirst: true, err: nats.ErrTimeout}, "eventtimeout",
		func(context.Context, Message) error { return nil })

	id, err := u.publishAndWait(t, "eventtimeout")
	require.NoError(t, err)
	u.requireNoRecordedError(t, id)
}

// requireNoRecordedError checks that no ERROR row shows up for the task
// within a wait long enough for the listener to have stored one.
func (u unrecordedSetup) requireNoRecordedError(t *testing.T, id string) {
	t.Helper()
	require.Never(t, func() bool {
		var n int
		err := u.db.QueryRow(context.Background(),
			`SELECT count(*) FROM message_states WHERE message_id=$1 AND state=$2`,
			id, int32(pb.State_STATE_ERROR)).Scan(&n)
		return err != nil || n > 0
	}, time.Second, 50*time.Millisecond, "no ERROR may contradict a state the stream may have stored")
}

func TestAStoredStateWhoseAckTimedOutIsNotContradicted(t *testing.T) {
	u := setupUnrecorded(t, refusingJetStream{refuse: pb.State_STATE_COMPLETED, storeFirst: true, err: nats.ErrTimeout}, "lostack",
		func(context.Context, Message) error { return nil })

	id, err := u.publishAndWait(t, "lostack")
	require.NoError(t, err, "the waiter gets the COMPLETED the stream stored")
	u.requireNoRecordedError(t, id)
}

func TestAnUnconfirmedReceivedStateStillRunsTheTask(t *testing.T) {
	var ran atomic.Bool
	u := setupUnrecorded(t, refusingJetStream{refuse: pb.State_STATE_RECEIVED, err: nats.ErrTimeout}, "receivedtimeout",
		func(context.Context, Message) error { ran.Store(true); return nil })

	id, err := u.publishAndWait(t, "receivedtimeout")
	require.NoError(t, err)
	require.True(t, ran.Load(), "a RECEIVED that may be stored must not leave the task unrun")
	u.requireNoRecordedError(t, id)
}

// A final success whose publish came back without a definite refusal, and
// which the server did not store, fails the task with the named cause.
// The waiter returns it well inside the bound. A deadline is the same
// kind of ambiguous error as a lost acknowledgement.
func TestALostFinalStateFailsTheTask(t *testing.T) {
	for _, tc := range []struct {
		method string
		err    error
	}{
		{method: "losttimeout", err: nats.ErrTimeout},
		{method: "lostdeadline", err: context.DeadlineExceeded},
	} {
		t.Run(tc.method, func(t *testing.T) {
			u := setupUnrecorded(t, refusingJetStream{refuse: pb.State_STATE_COMPLETED, err: tc.err}, tc.method,
				func(context.Context, Message) error { return nil })

			id, err := u.publishAndWait(t, tc.method)
			require.ErrorIs(t, err, ErrTaskFinishedNotSaved)
			require.EqualError(t, err, usercopy.TaskFinishedNotSaved)
			require.NotErrorIs(t, err, context.DeadlineExceeded)
			require.NotErrorIs(t, err, context.Canceled)
			require.NotContains(t, err.Error(), "nats:")
			u.requireRecordedError(t, id, usercopy.TaskFinishedNotSaved)
		})
	}
}

// A final ERROR that was not stored still ends the waiter with the
// command's cause. The lost acknowledgement is not a second diagnosis.
func TestALostFinalErrorEndsTheWaiterWithItsCause(t *testing.T) {
	const cause = "the provider has no prices for XYZ"
	u := setupUnrecorded(t, refusingJetStream{refuse: pb.State_STATE_ERROR, err: nats.ErrTimeout}, "losterror",
		func(context.Context, Message) error { return errors.New(cause) })

	id, err := u.publishAndWait(t, "losterror")
	require.EqualError(t, err, cause)
	require.NotErrorIs(t, err, context.DeadlineExceeded)
	u.requireRecordedError(t, id, cause)
}

// A final CANCELED that was not stored still ends the waiter as canceled.
func TestALostFinalCancelEndsTheWaiter(t *testing.T) {
	u := setupUnrecorded(t, refusingJetStream{refuse: pb.State_STATE_CANCELED, err: nats.ErrTimeout}, "lostcancel",
		func(context.Context, Message) error { return context.Canceled })

	id, err := u.publishAndWait(t, "lostcancel")
	require.EqualError(t, err, "canceled")
	require.NotErrorIs(t, err, context.DeadlineExceeded)
	u.requireRecordedCanceled(t, id)
}

// A follow-up read that fails, including a closed connection, ends the
// waiter with the confirmation cause. message_states gets no ERROR: the
// success may already be stored, and the unrecorded subject is what the
// listener copies.
func TestAFailedFinalReadEndsTheWaiterWithoutAnErrorRow(t *testing.T) {
	for _, tc := range []struct {
		method  string
		publish error
		read    error
	}{
		{method: "readclosed", publish: nats.ErrConnectionClosed, read: nats.ErrConnectionClosed},
		{method: "readfail", publish: nats.ErrTimeout, read: errors.New("read failed")},
	} {
		t.Run(tc.method, func(t *testing.T) {
			u := setupUnrecorded(t, refusingJetStream{
				refuse: pb.State_STATE_COMPLETED, err: tc.publish, readErr: tc.read,
			}, tc.method, func(context.Context, Message) error { return nil })

			id, err := u.publishAndWait(t, tc.method)
			require.ErrorIs(t, err, ErrTaskEndNotConfirmed)
			require.EqualError(t, err, usercopy.TaskEndNotConfirmed)
			require.NotErrorIs(t, err, context.DeadlineExceeded)
			require.NotErrorIs(t, err, context.Canceled)
			require.NotContains(t, err.Error(), "nats:")
			require.NotEqual(t, usercopy.TaskFinishedNotSaved, err.Error())
			u.requireNoRecordedError(t, id)
		})
	}
}

func TestATaskTheStreamRefusesAfterPublishedIsRecordedAsFailed(t *testing.T) {
	var ran atomic.Bool
	u := setupUnrecorded(t, refusingJetStream{}, "task",
		func(context.Context, Message) error { ran.Store(true); return nil })
	u.waiter.jt = refusingJetStream{JetStreamContext: u.waiter.jt, refuseTasks: true}

	id, err := u.publishAndWait(t, "task")
	require.ErrorIs(t, err, ErrStateNotRecorded)
	require.EqualError(t, err, usercopy.TaskStateNotRecorded)
	u.requireRecordedError(t, id, usercopy.TaskStateNotRecorded)
	require.False(t, ran.Load())
}

func TestAnAbortTheStreamRefusesNamesTheFullQueue(t *testing.T) {
	started := make(chan struct{})
	u := setupUnrecorded(t, refusingJetStream{}, "abort",
		func(ctx context.Context, _ Message) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		})
	u.waiter.jt = refusingJetStream{JetStreamContext: u.waiter.jt, refuseAborts: true}

	msg, err := NewMessage("unrecorded", "refuse", "abort", nil)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-started
		cancel()
	}()
	err = u.waiter.Publish(ctx, msg, WaitForCompletion())
	require.ErrorIs(t, err, ErrStateNotRecorded)
	require.EqualError(t, err, usercopy.TaskStateNotRecorded, "the NATS error goes to the log only")
}

func TestRefused(t *testing.T) {
	for _, err := range []error{
		errStreamFull,
		fmt.Errorf("error publishing state: %w", errStreamFull),
		&nats.APIError{Code: 400, ErrorCode: 10060, Description: "expected stream does not match"},
		nats.ErrNoStreamResponse,
		nats.ErrNoResponders,
	} {
		require.True(t, refused(err), "%v is a definite refusal", err)
	}
	for _, err := range []error{
		nats.ErrTimeout,
		context.DeadlineExceeded,
		context.Canceled,
		nats.ErrConnectionClosed,
		fmt.Errorf("error publishing state: %w", nats.ErrTimeout),
	} {
		require.False(t, refused(err), "%v may come after the server stored the message", err)
	}
}

func TestStreamFullMatchesAFullStreamsRefusal(t *testing.T) {
	test_helper.SetupEnvironment(t, &test_helper.Containers{NATS: true})
	nc, err := nats.Connect(environment.GetNATSURL())
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	require.NoError(t, err)

	cfg := StreamConfig()
	cfg.Name = fmt.Sprintf("foreverbull-full-%d", time.Now().UnixNano())
	cfg.Subjects = []string{cfg.Name + ".>"}
	cfg.MaxBytes = 1024
	cfg.Discard = nats.DiscardNew
	_, err = js.AddStream(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, js.DeleteStream(cfg.Name)) })

	_, err = js.Publish(cfg.Name+".a", make([]byte, 600))
	require.NoError(t, err)
	_, err = js.Publish(cfg.Name+".b", make([]byte, 600))
	require.Error(t, err)
	require.True(t, streamFull(err), "a full stream's refusal is %v", err)
	require.True(t, refused(err))
	require.False(t, streamFull(nats.ErrTimeout))
	require.False(t, streamFull(errors.New("maximum bytes exceeded")))
}

func TestUnrecordedState(t *testing.T) {
	for _, s := range []pb.State_State{pb.State_STATE_RECEIVED, pb.State_STATE_COMPLETED, pb.State_STATE_PUBLISHED} {
		got := unrecordedState(&pb.State{State: s}, errStreamFull)
		require.Equal(t, pb.State_STATE_ERROR, got.GetState(), s.String())
		require.Equal(t, usercopy.TaskStateNotRecorded, got.GetError(), s.String())
		require.NotNil(t, got.GetOccurredAt(), s.String())

		got = unrecordedState(&pb.State{State: s}, nats.ErrTimeout)
		require.Equal(t, usercopy.ServicesUnreachable, got.GetError(), s.String())
	}
	cause := "test error"
	failed := &pb.State{State: pb.State_STATE_ERROR, Error: &cause}
	require.Same(t, failed, unrecordedState(failed, errStreamFull))
	canceled := &pb.State{State: pb.State_STATE_CANCELED}
	require.Same(t, canceled, unrecordedState(canceled, errStreamFull))
}

func TestLostFinalState(t *testing.T) {
	cause := "the provider has no prices for XYZ"
	failed := &pb.State{State: pb.State_STATE_ERROR, Error: &cause}
	require.Same(t, failed, lostFinalState(failed))
	canceled := &pb.State{State: pb.State_STATE_CANCELED}
	require.Same(t, canceled, lostFinalState(canceled))

	got := lostFinalState(&pb.State{State: pb.State_STATE_COMPLETED})
	require.Equal(t, pb.State_STATE_ERROR, got.GetState())
	require.Equal(t, usercopy.TaskFinishedNotSaved, got.GetError())
	require.NotNil(t, got.GetOccurredAt())
}

func TestUnrecordedMessageID(t *testing.T) {
	id, err := unrecordedMessageID("foreverbull-unrecorded.backtest.session.run.m-1")
	require.NoError(t, err)
	require.Equal(t, "m-1", id)

	_, err = unrecordedMessageID("foreverbull.backtest.session.run.m-1.state")
	require.EqualError(t, err, `unrecorded subject "foreverbull.backtest.session.run.m-1.state" isn't foreverbull-unrecorded.<module>.<component>.<method>.<id>`)
}

func TestReportUnrecordedFailsWithoutACoreConnection(t *testing.T) {
	ns := &NatsSubscriber{module: "unrecorded"}
	msg := &message{msg: &pb.Message{Id: "m-1", Component: "refuse", Method: "completed"}}
	err := ns.reportUnrecorded(msg, &pb.State{State: pb.State_STATE_COMPLETED}, errStreamFull)
	require.EqualError(t, err, "error reporting an unrecorded state: no core NATS connection")
}

func TestWaitForCompletionNeedsACoreConnection(t *testing.T) {
	nn := &NATSStream{}
	_, _, err := WaitForCompletion()(nn, &message{msg: &pb.Message{Id: "m-1"}})
	require.EqualError(t, err, "error following the task: no core NATS connection for unrecorded states")
}
