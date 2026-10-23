package stream

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest"
	backtestStream "github.com/lhjnilsson/foreverbull/pkg/backtest/stream"
	common "github.com/lhjnilsson/foreverbull/pkg/pb"
	backtestpb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest/service"
	service "github.com/lhjnilsson/foreverbull/pkg/pb/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ErrBacktestMissing is returned when the named backtest does not exist.
// Callers should not ingest; point the user at `fbull backtest create`.
var ErrBacktestMissing = errors.New("backtest not found")

// SessionClient is the existing backtest SessionServicer (CreateExecution /
// RunExecution / StoreResult). Tests inject a mock.
type SessionClient interface {
	CreateExecution(ctx context.Context, req *servicePb.CreateExecutionRequest) (*servicePb.CreateExecutionResponse, error)
	RunExecution(ctx context.Context, executionID string) error
	StoreResult(ctx context.Context, executionID string) error
	Close(ctx context.Context) error
}

// SessionDialer opens a SessionClient for a session gRPC port.
type SessionDialer func(ctx context.Context, port int64) (SessionClient, error)

// SessionLauncher runs an experiment through CreateSession + session
// CreateExecution / RunExecution / StoreResult. It does not call Zipline
// directly.
type SessionLauncher struct {
	Store  backtest.Store
	Stream stream.Stream
	Dial   SessionDialer
}

var _ ExperimentLauncher = (*SessionLauncher)(nil)

func NewSessionLauncher(store backtest.Store, eventStream stream.Stream) *SessionLauncher {
	return &SessionLauncher{
		Store:  store,
		Stream: eventStream,
		Dial:   DialSession,
	}
}

func (l *SessionLauncher) Run(ctx context.Context, in RunInput) (*RunOutput, error) {
	if l.Store == nil {
		return nil, fmt.Errorf("backtest store is not configured")
	}
	if l.Stream == nil {
		return nil, fmt.Errorf("stream is not configured")
	}
	dial := l.Dial
	if dial == nil {
		dial = DialSession
	}

	bt, err := l.Store.GetBacktest(ctx, in.BacktestName)
	if err != nil {
		return nil, fmt.Errorf("%w: backtest %q; create and ingest it with `fbull backtest create` — experiments do not auto-ingest: %w",
			ErrBacktestMissing, in.BacktestName, err)
	}

	start, end := in.StartDate, in.EndDate
	if start == nil || end == nil {
		start, end, err = periodRange(bt)
		if err != nil {
			return nil, err
		}
	}

	session, err := l.Store.CreateSession(ctx, in.BacktestName)
	if err != nil {
		return nil, fmt.Errorf("create backtest session: %w", err)
	}

	msg, err := backtestStream.NewSessionRunCommand(in.BacktestName, session.Id)
	if err != nil {
		return nil, fmt.Errorf("create session run command: %w", err)
	}
	if err := l.Stream.Publish(ctx, msg, stream.WaitForCompletion()); err != nil {
		return nil, fmt.Errorf("start backtest session for %q (ingest it first with `fbull backtest create`): %w",
			in.BacktestName, err)
	}

	session, err = l.Store.GetSession(ctx, session.Id)
	if err != nil {
		return nil, fmt.Errorf("get backtest session: %w", err)
	}
	if session.Port == nil || session.GetPort() == 0 {
		return nil, fmt.Errorf("backtest %q session did not start; ingest it first with `fbull backtest create` — experiments do not auto-ingest",
			in.BacktestName)
	}

	client, err := dial(ctx, session.GetPort())
	if err != nil {
		return nil, fmt.Errorf("dial backtest session: %w", err)
	}
	defer func() { _ = client.Close(ctx) }()

	algo := in.Algorithm
	if algo == nil {
		algo = &service.Algorithm{}
	}

	created, err := client.CreateExecution(ctx, &servicePb.CreateExecutionRequest{
		Session:   session.Id,
		StartDate: start,
		EndDate:   end,
		Algorithm: algo,
	})
	if err != nil {
		return nil, fmt.Errorf("create execution: %w", err)
	}
	if created.GetExecution() == nil || created.GetExecution().GetId() == "" {
		return nil, fmt.Errorf("create execution: empty execution id")
	}

	out := &RunOutput{
		SessionID:   session.Id,
		ExecutionID: created.GetExecution().GetId(),
	}

	if err := client.RunExecution(ctx, out.ExecutionID); err != nil {
		return out, fmt.Errorf("run execution: %w", err)
	}
	if err := client.StoreResult(ctx, out.ExecutionID); err != nil {
		return out, fmt.Errorf("store execution result: %w", err)
	}

	return out, nil
}

func periodRange(bt *backtestpb.Backtest) (*common.Date, *common.Date, error) {
	if bt == nil || len(bt.Periods) == 0 {
		name := ""
		if bt != nil {
			name = bt.Name
		}
		// The agreed copy for a backtest without data is whole sentences, so it
		// ends with a period; the CLI prints it after ✖ as is.
		return nil, nil, fmt.Errorf("backtest '%[1]s' has no data to run. Experiments don't load data on their own, "+ //nolint:staticcheck // ST1005: agreed user-facing copy ends with a period
			"so delete it with `fbull backtest delete %[1]s`, then create it again with `fbull backtest create`.", name)
	}

	start := bt.Periods[0].StartDate
	end := bt.Periods[0].EndDate
	for _, p := range bt.Periods[1:] {
		if dateBefore(p.StartDate, start) {
			start = p.StartDate
		}
		if dateBefore(end, p.EndDate) {
			end = p.EndDate
		}
	}
	if start == nil || end == nil {
		return nil, nil, fmt.Errorf("backtest %q has no period dates; create and ingest it with `fbull backtest create`", bt.Name)
	}
	return start, end, nil
}

func dateBefore(a, b *common.Date) bool {
	if a == nil || b == nil {
		return false
	}
	if a.Year != b.Year {
		return a.Year < b.Year
	}
	if a.Month != b.Month {
		return a.Month < b.Month
	}
	return a.Day < b.Day
}

type grpcSessionClient struct {
	conn   *grpc.ClientConn
	client servicePb.SessionServicerClient
}

// DialSession connects to a backtest SessionServicer on localhost:port.
func DialSession(_ context.Context, port int64) (SessionClient, error) {
	conn, err := grpc.NewClient(
		fmt.Sprintf("localhost:%d", port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("dial session :%d: %w", port, err)
	}
	return &grpcSessionClient{
		conn:   conn,
		client: servicePb.NewSessionServicerClient(conn),
	}, nil
}

func (c *grpcSessionClient) CreateExecution(ctx context.Context, req *servicePb.CreateExecutionRequest) (*servicePb.CreateExecutionResponse, error) {
	return c.client.CreateExecution(ctx, req)
}

func (c *grpcSessionClient) RunExecution(ctx context.Context, executionID string) error {
	recv, err := c.client.RunExecution(ctx, &servicePb.RunExecutionRequest{ExecutionId: executionID})
	if err != nil {
		return err
	}
	for {
		_, err := recv.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func (c *grpcSessionClient) StoreResult(ctx context.Context, executionID string) error {
	_, err := c.client.StoreResult(ctx, &servicePb.StoreExecutionResultRequest{ExecutionId: executionID})
	return err
}

func (c *grpcSessionClient) Close(ctx context.Context) error {
	_, stopErr := c.client.StopServer(ctx, &servicePb.StopServerRequest{})
	closeErr := c.conn.Close()
	if stopErr != nil {
		return stopErr
	}
	return closeErr
}
