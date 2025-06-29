package repository_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	commonpb "github.com/lhjnilsson/foreverbull/pkg/pb"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type CalendarTest struct {
	suite.Suite

	conn *pgxpool.Pool
	uut  repository.Calendar
}

func (test *CalendarTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})
}

func (test *CalendarTest) SetupTest() {
	var err error
	test.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)
	err = repository.Recreate(context.Background(), test.conn)
	test.Require().NoError(err)

	test.uut = repository.Calendar{Conn: test.conn}
}

func (test *CalendarTest) TearDownTest() {
}

func TestCalendar(t *testing.T) {
	suite.Run(t, new(CalendarTest))
}

func (test *CalendarTest) TestStoreCalendarDate() {
	calendarCode := "XYNS"
	date := pb.CalendarDate{
		Date:  &commonpb.Date{Day: 1, Month: 1, Year: 2020},
		Open:  timestamppb.Now(),
		Close: timestamppb.Now(),
	}
	test.Run("successful", func() {
		err := test.uut.StoreCalendarDate(context.Background(), calendarCode, &date)
		test.Require().NoError(err)
	})
	test.Run("duplicate", func() {
		err := test.uut.StoreCalendarDate(context.Background(), calendarCode, &date)
		test.Require().Error(err)
		test.ErrorContains(err, "duplicate key value violates unique constraint")
	})
}

func (test *CalendarTest) TestListCalendarDates() {
	calendarCode := "XYNS"
	for day := range 10 {
		date := pb.CalendarDate{
			Date:  &commonpb.Date{Day: int32(day + 1), Month: 1, Year: 2020},
			Open:  timestamppb.Now(),
			Close: timestamppb.Now(),
		}
		err := test.uut.StoreCalendarDate(context.Background(), calendarCode, &date)
		test.Require().NoError(err)
	}
	type TestCase struct {
		Start          *commonpb.Date
		End            *commonpb.Date
		ExpectedLength int
	}
	testCases := []TestCase{
		{
			Start:          &commonpb.Date{Day: 1, Month: 1, Year: 2020},
			End:            &commonpb.Date{Day: 10, Month: 1, Year: 2020},
			ExpectedLength: 10,
		},
		{
			Start:          &commonpb.Date{Day: 1, Month: 1, Year: 2020},
			End:            &commonpb.Date{Day: 1, Month: 1, Year: 2020},
			ExpectedLength: 1,
		},
		{
			Start:          &commonpb.Date{Day: 1, Month: 1, Year: 2021},
			End:            &commonpb.Date{Day: 1, Month: 2, Year: 2021},
			ExpectedLength: 0,
		},
	}

	for i, tc := range testCases {
		test.Run(fmt.Sprintf("case %d", i), func() {
			dates, err := test.uut.ListCalendarDates(context.Background(), calendarCode, tc.Start, tc.End)
			test.Require().NoError(err)
			test.Len(dates, tc.ExpectedLength)
		})
	}
}

func (test *CalendarTest) TestCalendarHasTradingDates() {
	calendarCode := "XYNS"
	for day := range 20 {
		date := pb.CalendarDate{
			Date:  commonpb.GoTimeToDate(time.Now().Add(time.Hour * 24 * time.Duration(day))),
			Open:  timestamppb.Now(),
			Close: timestamppb.Now(),
		}
		err := test.uut.StoreCalendarDate(context.Background(), calendarCode, &date)
		test.Require().NoError(err)
	}
	assets := repository.Asset{
		Conn: test.conn,
	}
	err := assets.Store(context.Background(), "ABC", "ABC")
	test.Require().NoError(err)
	for day := range 10 {
		ohlc := repository.OHLC{
			Conn: test.conn,
		}
		err := ohlc.Store(context.Background(), "ABC", time.Now().Add(time.Hour*24*time.Duration(day)), 1.2, 1.3, 1.1, 1.2, 1000)
		test.Require().NoError(err)
	}
	type TestCase struct {
		CalendarCode string
		Symbols      []string
		FromDate     *commonpb.Date
		ToDate       *commonpb.Date
		Expected     bool
		ExpectedErr  error
	}
	testCases := []TestCase{
		{
			CalendarCode: calendarCode,
			Symbols:      []string{"ABC"},
			FromDate:     commonpb.GoTimeToDate(time.Now()),
			ToDate:       commonpb.GoTimeToDate(time.Now().Add(time.Hour * 24 * time.Duration(9))),
			Expected:     true,
			ExpectedErr:  nil,
		},
		{
			CalendarCode: calendarCode,
			Symbols:      []string{"ABC"},
			FromDate:     commonpb.GoTimeToDate(time.Now()),
			ToDate:       commonpb.GoTimeToDate(time.Now().Add(time.Hour * 24 * time.Duration(10))),
			Expected:     false,
			ExpectedErr:  repository.ErrMissingTradingDate,
		},
		{
			CalendarCode: calendarCode,
			Symbols:      []string{"ABC"},
			FromDate:     commonpb.GoTimeToDate(time.Now().Add(time.Hour * 24 * time.Duration(0))),
			ToDate:       commonpb.GoTimeToDate(time.Now().Add(time.Hour * 24 * time.Duration(30))),
			Expected:     false,
			ExpectedErr:  repository.ErrMissingCalendarDate,
		},
	}

	for i, tc := range testCases {
		test.Run(fmt.Sprintf("case %d", i), func() {
			missing, err := test.uut.CalendarHasTradingDates(context.Background(), tc.CalendarCode, tc.Symbols, tc.FromDate, tc.ToDate)
			test.Equal(tc.Expected, missing)
			if tc.ExpectedErr != nil {
				test.ErrorIs(err, tc.ExpectedErr)
			} else {
				test.NoError(err)
			}
		})
	}
}
