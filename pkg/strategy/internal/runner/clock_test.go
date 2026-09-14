package runner

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	commonpb "github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestFakeClock(t *testing.T) {
	t.Parallel()

	mon := SessionClose{
		Date:  &commonpb.Date{Year: 2024, Month: 3, Day: 4},
		Close: time.Date(2024, 3, 4, 16, 0, 0, 0, time.UTC),
	}
	tue := SessionClose{
		Date:  &commonpb.Date{Year: 2024, Month: 3, Day: 5},
		Close: time.Date(2024, 3, 5, 16, 0, 0, 0, time.UTC),
	}

	t.Run("next close after last", func(t *testing.T) {
		clock := NewFakeClock(mon.Close.Add(-time.Hour), mon, tue)
		got, err := clock.NextClose(context.Background(), "XNYS", nil)
		require.NoError(t, err)
		require.Equal(t, int32(4), got.Date.Day)

		got, err = clock.NextClose(context.Background(), "XNYS", mon.Date)
		require.NoError(t, err)
		require.Equal(t, int32(5), got.Date.Day)
	})

	t.Run("first deploy skips past closes", func(t *testing.T) {
		clock := NewFakeClock(tue.Close.Add(-time.Hour), mon, tue)
		got, err := clock.NextClose(context.Background(), "XNYS", nil)
		require.NoError(t, err)
		require.Equal(t, int32(5), got.Date.Day)
	})

	t.Run("wait until unblocks on advance", func(t *testing.T) {
		clock := NewFakeClock(mon.Close.Add(-time.Hour), mon)
		done := make(chan error, 1)
		go func() {
			done <- clock.WaitUntil(context.Background(), mon.Close)
		}()

		select {
		case err := <-done:
			t.Fatalf("WaitUntil returned before Advance: %v", err)
		case <-time.After(20 * time.Millisecond):
		}

		clock.Advance(mon.Close)
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(time.Second):
			t.Fatal("WaitUntil did not unblock")
		}
	})
}

type CalendarClockTest struct {
	suite.Suite

	conn *pgxpool.Pool
}

func TestCalendarClock(t *testing.T) {
	suite.Run(t, new(CalendarClockTest))
}

func (test *CalendarClockTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})
}

func (test *CalendarClockTest) SetupTest() {
	var err error
	test.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)

	_, err = test.conn.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS trading_calendar_date (
			code TEXT NOT NULL,
			date date NOT NULL,
			open timestamp NOT NULL,
			close timestamp NOT NULL,
			PRIMARY KEY (code, date)
		);
		TRUNCATE trading_calendar_date;
	`)
	test.Require().NoError(err)
}

func (test *CalendarClockTest) insert(code string, day int, close time.Time) {
	test.T().Helper()
	_, err := test.conn.Exec(context.Background(),
		`INSERT INTO trading_calendar_date (code, date, open, close) VALUES ($1, $2, $3, $4)`,
		code, time.Date(2024, 3, day, 0, 0, 0, 0, time.UTC).Format("2006-01-02"),
		close.Add(-6*time.Hour), close,
	)
	test.Require().NoError(err)
}

func (test *CalendarClockTest) TestNextClose() {
	monClose := time.Date(2024, 3, 4, 16, 0, 0, 0, time.UTC)
	tueClose := time.Date(2024, 3, 5, 16, 0, 0, 0, time.UTC)
	test.insert("XNYS", 4, monClose)
	test.insert("XNYS", 5, tueClose)

	frozen := time.Date(2024, 3, 4, 9, 0, 0, 0, time.UTC)
	clock := NewCalendarClock(test.conn)
	clock.now = func() time.Time { return frozen }

	got, err := clock.NextClose(context.Background(), "XNYS", nil)
	test.Require().NoError(err)
	test.Equal(int32(4), got.Date.Day)
	test.True(got.Close.Equal(monClose) || got.Close.UTC().Equal(monClose))

	got, err = clock.NextClose(context.Background(), "XNYS", &commonpb.Date{Year: 2024, Month: 3, Day: 4})
	test.Require().NoError(err)
	test.Equal(int32(5), got.Date.Day)

	_, err = clock.NextClose(context.Background(), "XNYS", &commonpb.Date{Year: 2024, Month: 3, Day: 5})
	test.ErrorIs(err, ErrNoSession)
}

func (test *CalendarClockTest) TestWaitUntilPast() {
	clock := NewCalendarClock(test.conn)
	err := clock.WaitUntil(context.Background(), time.Now().Add(-time.Second))
	test.Require().NoError(err)
}
