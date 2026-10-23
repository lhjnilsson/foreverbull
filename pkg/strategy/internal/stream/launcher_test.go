package stream_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest"
	common "github.com/lhjnilsson/foreverbull/pkg/pb"
	backtestpb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest/service"
	ss "github.com/lhjnilsson/foreverbull/pkg/strategy/internal/stream"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockStore struct {
	mock.Mock
}

func (m *mockStore) GetBacktest(ctx context.Context, name string) (*backtestpb.Backtest, error) {
	args := m.Called(ctx, name)
	bt, _ := args.Get(0).(*backtestpb.Backtest)
	return bt, args.Error(1)
}

func (m *mockStore) CreateSession(ctx context.Context, backtestName string) (*backtestpb.Session, error) {
	args := m.Called(ctx, backtestName)
	session, _ := args.Get(0).(*backtestpb.Session)
	return session, args.Error(1)
}

func (m *mockStore) GetSession(ctx context.Context, id string) (*backtestpb.Session, error) {
	args := m.Called(ctx, id)
	session, _ := args.Get(0).(*backtestpb.Session)
	return session, args.Error(1)
}

var _ backtest.Store = (*mockStore)(nil)

type mockSessionClient struct {
	mock.Mock
}

func (m *mockSessionClient) CreateExecution(ctx context.Context, req *servicePb.CreateExecutionRequest) (*servicePb.CreateExecutionResponse, error) {
	args := m.Called(ctx, req)
	rsp, _ := args.Get(0).(*servicePb.CreateExecutionResponse)
	return rsp, args.Error(1)
}

func (m *mockSessionClient) RunExecution(ctx context.Context, executionID string) error {
	args := m.Called(ctx, executionID)
	return args.Error(0)
}

func (m *mockSessionClient) StoreResult(ctx context.Context, executionID string) error {
	args := m.Called(ctx, executionID)
	return args.Error(0)
}

func (m *mockSessionClient) Close(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func spyPeriod() *backtestpb.Backtest_Period {
	return &backtestpb.Backtest_Period{
		StartDate: &common.Date{Year: 2020, Month: 1, Day: 2},
		EndDate:   &common.Date{Year: 2020, Month: 12, Day: 31},
	}
}

func TestSessionLauncherRun(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	port := int64(27001)
	session := &backtestpb.Session{Id: "session-1", Backtest: "spy", Port: &port}
	universe := &backtestpb.Backtest{Name: "spy", Periods: []*backtestpb.Backtest_Period{spyPeriod()}}

	store := new(mockStore)
	store.On("GetBacktest", mock.Anything, "spy").Return(universe, nil)
	store.On("CreateSession", mock.Anything, "spy").Return(&backtestpb.Session{Id: "session-1", Backtest: "spy"}, nil)
	store.On("GetSession", mock.Anything, "session-1").Return(session, nil)

	eventStream := new(stream.MockStream)
	eventStream.On("Publish", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	client := new(mockSessionClient)
	client.On("CreateExecution", mock.Anything, mock.Anything).Return(&servicePb.CreateExecutionResponse{
		Execution: &backtestpb.Execution{Id: "exec-1", Session: "session-1"},
	}, nil)
	client.On("RunExecution", mock.Anything, "exec-1").Return(nil)
	client.On("StoreResult", mock.Anything, "exec-1").Return(nil)
	client.On("Close", mock.Anything).Return(nil)

	launcher := &ss.SessionLauncher{
		Store:  store,
		Stream: eventStream,
		Dial: func(_ context.Context, gotPort int64) (ss.SessionClient, error) {
			require.Equal(t, port, gotPort)
			return client, nil
		},
	}

	out, err := launcher.Run(ctx, ss.RunInput{BacktestName: "spy"})
	require.NoError(t, err)
	require.Equal(t, "session-1", out.SessionID)
	require.Equal(t, "exec-1", out.ExecutionID)
	client.AssertCalled(t, "CreateExecution", mock.Anything, mock.Anything)
	client.AssertCalled(t, "RunExecution", mock.Anything, "exec-1")
	client.AssertCalled(t, "StoreResult", mock.Anything, "exec-1")
	eventStream.AssertCalled(t, "Publish", mock.Anything, mock.Anything, mock.Anything)
}

func TestSessionLauncherMissingBacktest(t *testing.T) {
	t.Parallel()

	store := new(mockStore)
	store.On("GetBacktest", mock.Anything, "missing").Return(nil, errors.New("no rows"))

	launcher := &ss.SessionLauncher{
		Store:  store,
		Stream: new(stream.MockStream),
	}

	_, err := launcher.Run(context.Background(), ss.RunInput{BacktestName: "missing"})
	require.Error(t, err)
	require.ErrorIs(t, err, ss.ErrBacktestMissing)
	require.Contains(t, err.Error(), "fbull backtest create")
	require.Contains(t, err.Error(), "do not auto-ingest")
}

func TestSessionLauncherNoPeriods(t *testing.T) {
	t.Parallel()

	store := new(mockStore)
	store.On("GetBacktest", mock.Anything, "spy").Return(&backtestpb.Backtest{Name: "spy"}, nil)

	launcher := &ss.SessionLauncher{
		Store:  store,
		Stream: new(stream.MockStream),
	}

	_, err := launcher.Run(context.Background(), ss.RunInput{BacktestName: "spy"})
	require.Error(t, err)
	require.Equal(t, "Backtest 'spy' has no data to run. Delete it with `fbull backtest delete spy`, "+
		"then create it again with `fbull backtest create`.", err.Error())
}

func TestSessionLauncherSessionRunFails(t *testing.T) {
	t.Parallel()

	store := new(mockStore)
	store.On("GetBacktest", mock.Anything, "spy").Return(&backtestpb.Backtest{
		Name:    "spy",
		Periods: []*backtestpb.Backtest_Period{spyPeriod()},
	}, nil)
	store.On("CreateSession", mock.Anything, "spy").Return(&backtestpb.Session{Id: "session-1"}, nil)

	eventStream := new(stream.MockStream)
	eventStream.On("Publish", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("no ingestions found"))

	launcher := &ss.SessionLauncher{
		Store:  store,
		Stream: eventStream,
	}

	_, err := launcher.Run(context.Background(), ss.RunInput{BacktestName: "spy"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "fbull backtest create")
	require.Contains(t, err.Error(), "no ingestions found")
}
