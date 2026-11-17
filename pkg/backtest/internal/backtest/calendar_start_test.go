package backtest

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	financeServicePb "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// earlyCalendar answers every read the way Zipline's calendar service
// answers one before its first session.
type earlyCalendar struct {
	financeServicePb.UnimplementedCalendarServer
}

func (earlyCalendar) ListCalendarDates(context.Context, *financeServicePb.ListCalendarDatesRequest) (*financeServicePb.ListCalendarDatesResponse, error) {
	return nil, status.Error(codes.OutOfRange, "1989-12-31 is before the XNYS calendar's first session 1990-01-02")
}

// A read before the calendar's first session is a CalendarStartError naming
// the requested start and that session, not an error listing sessions.
func TestZiplineSessionsBeforeTheCalendarIsACalendarStartError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer()
	financeServicePb.RegisterCalendarServer(server, earlyCalendar{})
	go server.Serve(listener) //nolint: errcheck
	t.Cleanup(server.Stop)

	zipline, err := NewZiplineEngine(context.Background(), ZiplineTarget{ControlAddress: listener.Addr().String()}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { zipline.Close() }) //nolint: errcheck

	_, err = zipline.Sessions(context.Background(),
		time.Date(1989, 12, 31, 0, 0, 0, 0, time.UTC), time.Date(1989, 12, 31, 0, 0, 0, 0, time.UTC))
	var calendarStart *engine.CalendarStartError
	require.ErrorAs(t, err, &calendarStart)
	require.Equal(t, "XNYS", calendarStart.Calendar)
	require.Equal(t, time.Date(1989, 12, 31, 0, 0, 0, 0, time.UTC), calendarStart.Start)
	require.Equal(t, time.Date(1990, 1, 2, 0, 0, 0, 0, time.UTC), calendarStart.FirstSession)
}

// Only OUT_OF_RANGE with the service's exact text is read as a start before
// the calendar. Anything else stays the error it was, including the end
// bound and the library's own DateOutOfBounds text.
func TestCalendarStartErrorReadsOnlyTheServicesStatus(t *testing.T) {
	t.Parallel()
	for name, err := range map[string]error{
		"other code":          status.Error(codes.Unknown, "1989-12-31 is before the XNYS calendar's first session 1990-01-02"),
		"other text":          status.Error(codes.OutOfRange, "start is before the calendar"),
		"trailing text":       status.Error(codes.OutOfRange, "1989-12-31 is before the XNYS calendar's first session 1990-01-02: more"),
		"not a date":          status.Error(codes.OutOfRange, "1989-13-40 is before the XNYS calendar's first session 1990-01-02"),
		"end bound":           status.Error(codes.OutOfRange, "the XNYS calendar ends on 2027-11-02"),
		"not a status":        errors.New("1989-12-31 is before the XNYS calendar's first session 1990-01-02"),
		"library's own error": status.Error(codes.Unknown, "Exception calling application: Parameter `start` received as '1989-12-31 00:00:00' although it cannot be earlier than the first session of calendar 'XNYS' ('1990-01-02 00:00:00')."),
	} {
		_, ok := calendarStartError(err)
		require.False(t, ok, name)
	}
	calendarStart, ok := calendarStartError(status.Error(codes.OutOfRange, "1989-12-31 is before the XNYS calendar's first session 1990-01-02"))
	require.True(t, ok)
	require.Equal(t, &engine.CalendarStartError{
		Calendar:     "XNYS",
		Start:        time.Date(1989, 12, 31, 0, 0, 0, 0, time.UTC),
		FirstSession: time.Date(1990, 1, 2, 0, 0, 0, 0, time.UTC),
	}, calendarStart)
}
