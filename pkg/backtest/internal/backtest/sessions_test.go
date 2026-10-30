package backtest

import (
	"context"
	"net"
	"testing"
	"time"

	commonPb "github.com/lhjnilsson/foreverbull/pkg/pb"
	financePb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	financeServicePb "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

// fakeCalendar answers ListCalendarDates like Zipline's calendar service,
// which shares the engine's control port.
type fakeCalendar struct {
	financeServicePb.UnimplementedCalendarServer

	requests []*financeServicePb.ListCalendarDatesRequest
}

func (f *fakeCalendar) ListCalendarDates(_ context.Context, req *financeServicePb.ListCalendarDatesRequest) (*financeServicePb.ListCalendarDatesResponse, error) {
	f.requests = append(f.requests, req)
	// Good Friday 2024-03-29 is not a session.
	days := []string{"2024-03-27", "2024-03-28", "2024-04-01"}
	rsp := &financeServicePb.ListCalendarDatesResponse{}
	for _, day := range days {
		rsp.Dates = append(rsp.Dates, &financePb.CalendarDate{Date: commonPb.DateStringToDate(day)})
	}
	return rsp, nil
}

func TestZiplineSessionsReadsTheXNYSCalendarOnTheControlConnection(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	fake := &fakeCalendar{}
	server := grpc.NewServer()
	financeServicePb.RegisterCalendarServer(server, fake)
	go server.Serve(listener) //nolint: errcheck
	t.Cleanup(server.Stop)

	zipline, err := NewZiplineEngine(context.Background(), ZiplineTarget{ControlAddress: listener.Addr().String()}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { zipline.Close() }) //nolint: errcheck

	start := time.Date(2024, 3, 27, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)
	sessions, err := zipline.Sessions(context.Background(), start, end)
	require.NoError(t, err)
	require.Equal(t, []time.Time{
		time.Date(2024, 3, 27, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 3, 28, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC),
	}, sessions)

	require.Len(t, fake.requests, 1)
	require.Equal(t, "XNYS", fake.requests[0].GetCode())
	require.Equal(t, "2024-03-27", commonPb.DateToDateString(fake.requests[0].GetStart()))
	require.Equal(t, "2024-04-01", commonPb.DateToDateString(fake.requests[0].GetEnd()))
}
