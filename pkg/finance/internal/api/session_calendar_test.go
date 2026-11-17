package api_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/internal/usercopy"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/api"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	workerpb "github.com/lhjnilsson/foreverbull/pkg/pb/service"
	"github.com/lhjnilsson/foreverbull/pkg/service/worker"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const calendarNotConfigured = "trading: this broker wasn't set up with the XNYS trading calendar, so it won't trade. "

func TestXNYSCalendarWithoutAConnection(t *testing.T) {
	t.Parallel()
	_, err := api.NewXNYSCalendar(nil).LastSession(context.Background(), time.Date(2026, time.November, 16, 15, 0, 0, 0, time.UTC))
	require.ErrorIs(t, err, worker.ErrCalendarNotConfigured)
	require.NotErrorIs(t, err, worker.ErrNoCompletedSession)
	require.Equal(t, "Please report it at https://github.com/lhjnilsson/foreverbull/issues with this message.", usercopy.ReportIt)
	require.EqualError(t, err, calendarNotConfigured+usercopy.ReportIt)
}

type queryFailure struct {
	err error
}

func (q queryFailure) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, q.err
}

func (q queryFailure) QueryRow(context.Context, string, ...interface{}) pgx.Row {
	return nil
}

func (q queryFailure) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, q.err
}

func TestXNYSCalendarHidesAQueryFailure(t *testing.T) {
	t.Parallel()
	cause := errors.New("pq: password=secret relation trading_calendar_date")
	_, err := api.NewXNYSCalendar(queryFailure{err: cause}).LastSession(context.Background(), time.Date(2026, time.November, 13, 22, 0, 0, 0, time.UTC))
	require.ErrorIs(t, err, worker.ErrCalendarRead)
	require.ErrorIs(t, err, cause)
	require.EqualError(t, err, "trading: foreverbull couldn't read its XNYS trading calendar, so it won't trade. Run `fbenv status` to check that the environment is running, then run `fbull trading run` again. If it repeats, please report it at https://github.com/lhjnilsson/foreverbull/issues with this message.")
	require.NotContains(t, err.Error(), "password")
	require.NotContains(t, err.Error(), "relation")
}

func TestXNYSCalendarReturnsTheContextError(t *testing.T) {
	t.Parallel()
	asOf := time.Date(2026, time.November, 13, 22, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		ctx  func() (context.Context, context.CancelFunc)
		err  error
	}{
		{
			name: "canceled",
			ctx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, cancel
			},
			err: context.Canceled,
		},
		{
			name: "deadline",
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			},
			err: context.DeadlineExceeded,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := tc.ctx()
			defer cancel()
			_, err := api.NewXNYSCalendar(queryFailure{err: tc.err}).LastSession(ctx, asOf)
			require.ErrorIs(t, err, tc.err)
			require.NotErrorIs(t, err, worker.ErrCalendarRead)
			require.EqualError(t, err, tc.err.Error())
		})
	}
}

func TestXNYSCalendarStripsAWrappedCancel(t *testing.T) {
	t.Parallel()
	driver := fmt.Errorf("pq: password=secret: %w", context.Canceled)
	_, err := api.NewXNYSCalendar(queryFailure{err: driver}).LastSession(
		context.Background(),
		time.Date(2026, time.November, 13, 22, 0, 0, 0, time.UTC),
	)
	require.Equal(t, context.Canceled, err)
	require.NotErrorIs(t, err, worker.ErrCalendarRead)
	require.NotContains(t, err.Error(), "pq:")
	require.NotContains(t, err.Error(), "password")
}

type stubBars struct{}

func (stubBars) List(context.Context, string, []string, time.Time, int) ([]*pb.OHLC, error) {
	return nil, nil
}

func TestConfigureRefusesAMissingCalendar(t *testing.T) {
	test_helper.SetupEnv(t)
	listener := bufconn.Listen(1024 * 1024)
	server, _, err := api.NewGRPCSessionServer(new(supplier.MockTrading), api.WithBars(stubBars{}, "yahoo"))
	require.NoError(t, err)
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- server.Serve(listener)
	}()
	t.Cleanup(func() {
		server.Stop()
		// Serve returns nil once Stop has closed the listener. A real
		// accept failure is the only error left on the channel.
		if err := <-serveErr; err != nil {
			t.Errorf("calendar server: %v", err)
		}
		_ = listener.Close()
	})

	resolver.SetDefaultScheme("passthrough")
	conn, err := grpc.NewClient(listener.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	client := servicePb.NewSessionServicerClient(conn)
	_, err = client.Configure(context.Background(), &servicePb.ConfigureRequest{
		Algorithm: &workerpb.Algorithm{},
	})
	st := status.Convert(err)
	require.Equal(t, codes.FailedPrecondition, st.Code())
	require.EqualError(t, errors.New(st.Message()), calendarNotConfigured+usercopy.ReportIt)
}
