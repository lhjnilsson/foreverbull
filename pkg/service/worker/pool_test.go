package worker_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	// Tests build 16:00 America/New_York closes. The image may have no zoneinfo.
	_ "time/tzdata"

	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/internal/usercopy"
	finance_pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/service/service"
	"github.com/lhjnilsson/foreverbull/pkg/service/worker"
	"github.com/stretchr/testify/suite"
	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/rep"
	"go.nanomsg.org/mangos/v3/protocol/req"
	_ "go.nanomsg.org/mangos/v3/transport/all" // for mangos transport.
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type PoolTest struct {
	suite.Suite
}

func (test *PoolTest) SetupSuite() {
	test_helper.SetupEnv(test.T())
}

func TestPool(t *testing.T) {
	suite.Run(t, new(PoolTest))
}

func (test *PoolTest) TestConfigureDoesNotSendADatabaseURL() {
	algo, _ := test_helper.WorkerSimulator(test.T(), &test_helper.WorkerFunction{Name: "test"})
	pool, err := worker.NewPool(context.TODO(), algo)
	test.Require().NoError(err)
	defer pool.Close()

	configuration := pool.Configure()
	test.Empty(configuration.GetDatabaseURL())
	test.NotContains(configuration.String(), "postgres://")
	test.NotContains(configuration.String(), "foreverbull@")
}

type recordingBars struct {
	calls    int
	sizes    []int
	provider string
	asOf     time.Time
	limit    int
	bars     []*finance_pb.OHLC
	err      error
	// deliverRequested returns one bar per symbol in the call. The chunk
	// test uses it so a presence check is not an empty delivery.
	deliverRequested bool
}

func (r *recordingBars) List(_ context.Context, provider string, symbols []string, asOf time.Time, limit int) ([]*finance_pb.OHLC, error) {
	r.calls++
	r.sizes = append(r.sizes, len(symbols))
	r.provider = provider
	r.asOf = asOf
	r.limit = limit
	if r.err != nil {
		return nil, r.err
	}
	if r.deliverRequested {
		out := make([]*finance_pb.OHLC, 0, len(symbols))
		for _, symbol := range symbols {
			out = append(out, &finance_pb.OHLC{Symbol: symbol, Timestamp: timestamppb.New(asOf)})
		}
		return out, nil
	}
	return r.bars, nil
}

func (test *PoolTest) TestProcessSendsBarsWithoutADatabaseURL() {
	when := time.Date(2024, time.March, 9, 15, 4, 5, 0, time.UTC)
	source := &recordingBars{bars: []*finance_pb.OHLC{
		&finance_pb.OHLC{Symbol: "AAPL", Close: 10, Timestamp: timestamppb.New(when)},
		&finance_pb.OHLC{Symbol: "MSFT", Close: 11, Timestamp: timestamppb.New(when)},
	}}

	var mu sync.Mutex
	var requests []*servicePb.WorkerRequest
	handler := func(req *servicePb.WorkerRequest) *servicePb.WorkerResponse {
		mu.Lock()
		defer mu.Unlock()
		requests = append(requests, proto.Clone(req).(*servicePb.WorkerRequest))
		return &servicePb.WorkerResponse{}
	}
	algo, runner := test_helper.WorkerSimulator(test.T(), &test_helper.WorkerFunction{
		CB: handler, Name: "handle_data", Parallel: true,
	})
	pool, err := worker.NewPool(context.TODO(), algo, worker.WithBars(source, "stooq"))
	test.Require().NoError(err)
	defer pool.Close()

	configuration := pool.Configure()
	test.Empty(configuration.GetDatabaseURL())
	configBytes, err := proto.Marshal(configuration)
	test.Require().NoError(err)
	test.NotContains(string(configBytes), "postgres://")

	socket, err := rep.NewSocket()
	test.Require().NoError(err)
	defer socket.Close()
	test.Require().NoError(socket.Dial(fmt.Sprintf("tcp://localhost:%d", configuration.BrokerPort)))
	test.Require().NoError(socket.SetOption(mangos.OptionRecvDeadline, time.Second))
	test.Require().NoError(socket.SetOption(mangos.OptionSendDeadline, time.Second))
	go runner(socket)

	portfolio := &finance_pb.Portfolio{Timestamp: timestamppb.New(when)}
	_, err = pool.Process(context.TODO(), when.Add(time.Hour), []string{"AAPL", "MSFT"}, portfolio)
	test.Require().NoError(err)
	test.Equal("stooq", source.provider)
	test.Equal(252, source.limit)
	test.True(when.Equal(source.asOf))
	test.Require().Len(requests, 2)
	bySymbol := map[string]string{}
	for _, req := range requests {
		test.Require().Len(req.GetBars(), 1)
		bySymbol[req.GetSymbols()[0]] = req.GetBars()[0].GetSymbol()
	}
	test.Equal("AAPL", bySymbol["AAPL"])
	test.Equal("MSFT", bySymbol["MSFT"])
	raw, err := proto.Marshal(requests[0])
	test.Require().NoError(err)
	test.NotContains(string(raw), "postgres://")
	test.NotContains(string(raw), "foreverbull@")
}

func (test *PoolTest) TestProcessChunksBarReads() {
	source := &recordingBars{deliverRequested: true}
	handler := func(_ *servicePb.WorkerRequest) *servicePb.WorkerResponse {
		return &servicePb.WorkerResponse{}
	}
	algo, runner := test_helper.WorkerSimulator(test.T(), &test_helper.WorkerFunction{CB: handler, Name: "handle_data"})
	pool, err := worker.NewPool(context.TODO(), algo, worker.WithBars(source, "yahoo"))
	test.Require().NoError(err)
	defer pool.Close()

	configuration := pool.Configure()
	socket, err := rep.NewSocket()
	test.Require().NoError(err)
	defer socket.Close()
	test.Require().NoError(socket.Dial(fmt.Sprintf("tcp://localhost:%d", configuration.BrokerPort)))
	test.Require().NoError(socket.SetOption(mangos.OptionRecvDeadline, time.Second))
	test.Require().NoError(socket.SetOption(mangos.OptionSendDeadline, time.Second))
	go runner(socket)

	symbols := make([]string, 501)
	for i := range symbols {
		symbols[i] = fmt.Sprintf("S%d", i)
	}
	_, err = pool.Process(context.TODO(), time.Now(), symbols, &finance_pb.Portfolio{})
	test.Require().NoError(err)
	test.Equal(2, source.calls)
	test.Equal([]int{500, 1}, source.sizes)
}

func (test *PoolTest) TestProcessReturnsABarLoadErrorBeforeSending() {
	source := &recordingBars{err: fmt.Errorf("read failed")}
	algo, _ := test_helper.WorkerSimulator(test.T(), &test_helper.WorkerFunction{Name: "handle_data"})
	pool, err := worker.NewPool(context.TODO(), algo, worker.WithBars(source, "yahoo"))
	test.Require().NoError(err)
	defer pool.Close()

	_, err = pool.Process(context.TODO(), time.Now(), []string{"AAPL"}, &finance_pb.Portfolio{})
	test.Require().EqualError(err, "load bars: read failed")
	test.NotContains(err.Error(), "postgres://")
}

func (test *PoolTest) TestProcessFailsWhenARequestedSymbolHasNoBars() {
	// 01:00 at UTC+2 is 23:00 the previous UTC day. The named date is that
	// UTC day, not the wall clock and not the timestamp argument when a
	// portfolio time is set.
	loc := time.FixedZone("CEST", 2*60*60)
	wall := time.Date(2024, time.June, 3, 1, 0, 0, 0, loc)
	source := &recordingBars{bars: []*finance_pb.OHLC{
		{Symbol: "AAPL", Close: 10, Timestamp: timestamppb.New(wall)},
	}}
	algo, _ := test_helper.WorkerSimulator(test.T(), &test_helper.WorkerFunction{Name: "handle_data"})
	pool, err := worker.NewPool(context.TODO(), algo, worker.WithBars(source, "yahoo"))
	test.Require().NoError(err)
	defer pool.Close()

	_, err = pool.Process(context.TODO(), wall, []string{"MSFT", "AAPL", "NVDA", "IBM", "MSFT"}, nil)
	test.Require().EqualError(err, "no bars for MSFT, NVDA and IBM on 2024-06-02")
	test.NotContains(err.Error(), "AAPL")
	test.Equal(1, source.calls)

	portfolio := &finance_pb.Portfolio{Timestamp: timestamppb.New(wall)}
	other := time.Date(2020, time.January, 2, 0, 0, 0, 0, time.UTC)
	_, err = pool.Process(context.TODO(), other, []string{"IBM"}, portfolio)
	test.Require().EqualError(err, "no bars for IBM on 2024-06-02")
	test.Equal(2, source.calls)
}

// asOf is 17:00 in New York on 2026-11-13, after the 16:30 close, so the
// as-of session is that date. sessionBar is the Yahoo open stamp of that
// session, and olderBar is the open stamp of the previous one.
func asOfSessionFixtures() (asOf, sessionBar, olderBar time.Time) {
	asOf = time.Date(2026, time.November, 13, 22, 0, 0, 0, time.UTC)
	sessionBar = time.Date(2026, time.November, 13, 14, 30, 0, 0, time.UTC)
	olderBar = time.Date(2026, time.November, 12, 14, 30, 0, 0, time.UTC)
	return asOf, sessionBar, olderBar
}

const missingAsOfBarMessage = "trading: yahoo has no price for AAPL on 2026-11-13, the latest finished XNYS session, so foreverbull won't trade on older prices. Run `fbull trading run` again once yahoo has that day's prices."

var newYork = func() *time.Location {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		panic(err)
	}
	return location
}()

// nySession is one XNYS row: the date, and a 16:00 New York close.
func nySession(year int, month time.Month, day int) worker.CalendarSession {
	return worker.CalendarSession{
		Date:  time.Date(year, month, day, 0, 0, 0, 0, time.UTC),
		Close: time.Date(year, month, day, 16, 0, 0, 0, newYork),
	}
}

type memoryCalendar struct {
	sessions []worker.CalendarSession
}

type failingCalendar struct {
	err error
}

func (f failingCalendar) LastSession(context.Context, time.Time) (time.Time, error) {
	return time.Time{}, f.err
}

func (m memoryCalendar) LastSession(_ context.Context, asOf time.Time) (time.Time, error) {
	return worker.LastFinishedSession(m.sessions, asOf)
}

// fixtureCalendar is the week of 2026-11-13, a Friday. Saturday and Sunday
// are not rows.
func fixtureCalendar() worker.SessionCalendar {
	return memoryCalendar{sessions: []worker.CalendarSession{
		nySession(2026, time.November, 11),
		nySession(2026, time.November, 12),
		nySession(2026, time.November, 13),
	}}
}

func (test *PoolTest) TestTradingFailsWhenTheAsOfBarIsMissing() {
	asOf, sessionBar, olderBar := asOfSessionFixtures()
	source := &recordingBars{bars: []*finance_pb.OHLC{
		{Symbol: "AAPL", Close: 10, Timestamp: timestamppb.New(olderBar)},
		{Symbol: "MSFT", Close: 11, Timestamp: timestamppb.New(sessionBar)},
		{Symbol: "IBM", Close: 12, Timestamp: timestamppb.New(olderBar)},
	}}
	algo, _ := test_helper.WorkerSimulator(test.T(), &test_helper.WorkerFunction{Name: "handle_data"})
	pool, err := worker.NewPool(context.TODO(), algo, worker.WithBars(source, "yahoo"), worker.WithSessionBarRequired(fixtureCalendar()))
	test.Require().NoError(err)
	defer pool.Close()

	// The portfolio clock is the as-of. The timestamp argument is not.
	portfolio := &finance_pb.Portfolio{Timestamp: timestamppb.New(asOf)}
	other := time.Date(2020, time.January, 2, 0, 0, 0, 0, time.UTC)
	_, err = pool.Process(context.TODO(), other, []string{"AAPL", "MSFT"}, portfolio)
	test.Require().ErrorIs(err, worker.ErrMissingAsOfBar)
	test.Require().EqualError(err, missingAsOfBarMessage)
	test.NotContains(err.Error(), "MSFT")
	test.Equal(1, source.calls)

	_, err = pool.Process(context.TODO(), asOf, []string{"IBM", "AAPL"}, portfolio)
	test.Require().ErrorIs(err, worker.ErrMissingAsOfBar)
	test.Require().EqualError(err, "trading: yahoo has no prices for IBM and AAPL on 2026-11-13, the latest finished XNYS session, so foreverbull won't trade on older prices. Run `fbull trading run` again once yahoo has that day's prices.")
	test.Equal(2, source.calls)
}

func (test *PoolTest) TestTradingAcceptsTheLastCompletedSessionBeforeTheClose() {
	// 10:00 in New York. Friday's session has not closed, so the as-of
	// session is Thursday. A Thursday bar is enough. Requiring Friday
	// would refuse every run before 16:30.
	morning := time.Date(2026, time.November, 13, 15, 0, 0, 0, time.UTC)
	thursday := time.Date(2026, time.November, 12, 14, 30, 0, 0, time.UTC)
	wednesday := time.Date(2026, time.November, 11, 14, 30, 0, 0, time.UTC)

	stale := &recordingBars{bars: []*finance_pb.OHLC{
		{Symbol: "AAPL", Close: 9, Timestamp: timestamppb.New(wednesday)},
	}}
	algo, _ := test_helper.WorkerSimulator(test.T(), &test_helper.WorkerFunction{Name: "handle_data"})
	pool, err := worker.NewPool(context.TODO(), algo, worker.WithBars(stale, "yahoo"), worker.WithSessionBarRequired(fixtureCalendar()))
	test.Require().NoError(err)
	defer pool.Close()

	portfolio := &finance_pb.Portfolio{Timestamp: timestamppb.New(morning)}
	_, err = pool.Process(context.TODO(), morning, []string{"AAPL"}, portfolio)
	test.Require().ErrorIs(err, worker.ErrMissingAsOfBar)
	test.Require().EqualError(err, "trading: yahoo has no price for AAPL on 2026-11-12, the latest finished XNYS session, so foreverbull won't trade on older prices. Run `fbull trading run` again once yahoo has that day's prices.")

	source := &recordingBars{bars: []*finance_pb.OHLC{
		{Symbol: "AAPL", Close: 9, Timestamp: timestamppb.New(wednesday)},
		{Symbol: "AAPL", Close: 10, Timestamp: timestamppb.New(thursday)},
	}}
	var mu sync.Mutex
	var got time.Time
	handler := func(req *servicePb.WorkerRequest) *servicePb.WorkerResponse {
		mu.Lock()
		defer mu.Unlock()
		for _, bar := range req.GetBars() {
			when := bar.GetTimestamp().AsTime()
			if when.After(got) {
				got = when
			}
		}
		return &servicePb.WorkerResponse{}
	}
	live, socket := test.startBarsPool(source, "yahoo", fixtureCalendar(), handler)
	defer socket.Close()
	defer live.Close()

	_, err = live.Process(context.TODO(), morning, []string{"AAPL"}, portfolio)
	test.Require().NoError(err)
	test.True(thursday.Equal(got))
}

func (test *PoolTest) TestTradingProceedsWhenTheAsOfBarIsPresent() {
	asOf, sessionBar, olderBar := asOfSessionFixtures()
	source := &recordingBars{bars: []*finance_pb.OHLC{
		{Symbol: "AAPL", Close: 9, Timestamp: timestamppb.New(olderBar)},
		{Symbol: "AAPL", Close: 10, Timestamp: timestamppb.New(sessionBar)},
		{Symbol: "MSFT", Close: 11, Timestamp: timestamppb.New(sessionBar)},
	}}
	var mu sync.Mutex
	var requests []*servicePb.WorkerRequest
	handler := func(req *servicePb.WorkerRequest) *servicePb.WorkerResponse {
		mu.Lock()
		defer mu.Unlock()
		requests = append(requests, proto.Clone(req).(*servicePb.WorkerRequest))
		return &servicePb.WorkerResponse{}
	}
	pool, socket := test.startBarsPool(source, "yahoo", fixtureCalendar(), handler)
	defer socket.Close()
	defer pool.Close()

	portfolio := &finance_pb.Portfolio{Timestamp: timestamppb.New(asOf)}
	_, err := pool.Process(context.TODO(), asOf, []string{"AAPL", "MSFT"}, portfolio)
	test.Require().NoError(err)
	test.Require().Len(requests, 1)
	test.Equal([]string{"AAPL", "MSFT"}, requests[0].GetSymbols())
	test.Require().Len(requests[0].GetBars(), 3)
	var newest time.Time
	for _, bar := range requests[0].GetBars() {
		if bar.GetSymbol() != "AAPL" {
			continue
		}
		when := bar.GetTimestamp().AsTime()
		if when.After(newest) {
			newest = when
		}
	}
	test.True(sessionBar.Equal(newest))
}

func (test *PoolTest) TestBacktestHandsAnOlderBar() {
	asOf, sessionBar, olderBar := asOfSessionFixtures()
	source := &recordingBars{bars: []*finance_pb.OHLC{
		{Symbol: "AAPL", Close: 10, Timestamp: timestamppb.New(olderBar)},
		{Symbol: "MSFT", Close: 11, Timestamp: timestamppb.New(sessionBar)},
	}}
	var mu sync.Mutex
	var requests []*servicePb.WorkerRequest
	handler := func(req *servicePb.WorkerRequest) *servicePb.WorkerResponse {
		mu.Lock()
		defer mu.Unlock()
		requests = append(requests, proto.Clone(req).(*servicePb.WorkerRequest))
		return &servicePb.WorkerResponse{}
	}
	pool, socket := test.startBarsPool(source, "yahoo", nil, handler)
	defer socket.Close()
	defer pool.Close()

	portfolio := &finance_pb.Portfolio{Timestamp: timestamppb.New(asOf)}
	_, err := pool.Process(context.TODO(), asOf, []string{"AAPL", "MSFT"}, portfolio)
	test.Require().NoError(err)
	test.Require().Len(requests, 1)
	test.Require().Len(requests[0].GetBars(), 2)
	test.Equal("AAPL", requests[0].GetBars()[0].GetSymbol())
	test.True(olderBar.Equal(requests[0].GetBars()[0].GetTimestamp().AsTime()))
}

func (test *PoolTest) TestTradingMondayMorningUsesFriday() {
	// 10:00 Monday. Friday's close has finished. Saturday and Sunday are
	// not sessions, so the as-of session is Friday, and a Friday bar is
	// enough.
	morning := time.Date(2026, time.November, 16, 10, 0, 0, 0, newYork)
	friday := time.Date(2026, time.November, 13, 14, 30, 0, 0, time.UTC)
	thursday := time.Date(2026, time.November, 12, 14, 30, 0, 0, time.UTC)
	calendar := memoryCalendar{sessions: []worker.CalendarSession{
		nySession(2026, time.November, 13),
		nySession(2026, time.November, 16),
	}}

	stale := &recordingBars{bars: []*finance_pb.OHLC{
		{Symbol: "AAPL", Close: 9, Timestamp: timestamppb.New(thursday)},
	}}
	algo, _ := test_helper.WorkerSimulator(test.T(), &test_helper.WorkerFunction{Name: "handle_data"})
	pool, err := worker.NewPool(context.TODO(), algo, worker.WithBars(stale, "yahoo"), worker.WithSessionBarRequired(calendar))
	test.Require().NoError(err)
	defer pool.Close()

	portfolio := &finance_pb.Portfolio{Timestamp: timestamppb.New(morning)}
	_, err = pool.Process(context.TODO(), morning, []string{"AAPL"}, portfolio)
	test.Require().ErrorIs(err, worker.ErrMissingAsOfBar)
	test.Require().EqualError(err, missingAsOfBarMessage)

	source := &recordingBars{bars: []*finance_pb.OHLC{
		{Symbol: "AAPL", Close: 9, Timestamp: timestamppb.New(thursday)},
		{Symbol: "AAPL", Close: 10, Timestamp: timestamppb.New(friday)},
	}}
	var got time.Time
	handler := func(req *servicePb.WorkerRequest) *servicePb.WorkerResponse {
		for _, bar := range req.GetBars() {
			when := bar.GetTimestamp().AsTime()
			if when.After(got) {
				got = when
			}
		}
		return &servicePb.WorkerResponse{}
	}
	live, socket := test.startBarsPool(source, "yahoo", calendar, handler)
	defer socket.Close()
	defer live.Close()

	_, err = live.Process(context.TODO(), morning, []string{"AAPL"}, portfolio)
	test.Require().NoError(err)
	test.True(friday.Equal(got))
}

func (test *PoolTest) TestTradingMorningAfterThanksgivingNamesThePriorSession() {
	// 2026-11-26 is Thanksgiving and is not a session. Friday morning the
	// as-of session is Wednesday 2026-11-25, not the holiday.
	morning := time.Date(2026, time.November, 27, 10, 0, 0, 0, newYork)
	tuesday := time.Date(2026, time.November, 24, 14, 30, 0, 0, time.UTC)
	calendar := memoryCalendar{sessions: []worker.CalendarSession{
		nySession(2026, time.November, 25),
		nySession(2026, time.November, 27),
	}}
	source := &recordingBars{bars: []*finance_pb.OHLC{
		{Symbol: "AAPL", Close: 9, Timestamp: timestamppb.New(tuesday)},
	}}
	algo, _ := test_helper.WorkerSimulator(test.T(), &test_helper.WorkerFunction{Name: "handle_data"})
	pool, err := worker.NewPool(context.TODO(), algo, worker.WithBars(source, "yahoo"), worker.WithSessionBarRequired(calendar))
	test.Require().NoError(err)
	defer pool.Close()

	portfolio := &finance_pb.Portfolio{Timestamp: timestamppb.New(morning)}
	_, err = pool.Process(context.TODO(), morning, []string{"AAPL"}, portfolio)
	test.Require().ErrorIs(err, worker.ErrMissingAsOfBar)
	test.Require().EqualError(err, "trading: yahoo has no price for AAPL on 2026-11-25, the latest finished XNYS session, so foreverbull won't trade on older prices. Run `fbull trading run` again once yahoo has that day's prices.")
}

func (test *PoolTest) TestTradingSaturdayAfterTheCloseNamesFriday() {
	saturday := time.Date(2026, time.November, 14, 17, 0, 0, 0, newYork)
	thursday := time.Date(2026, time.November, 12, 14, 30, 0, 0, time.UTC)
	source := &recordingBars{bars: []*finance_pb.OHLC{
		{Symbol: "AAPL", Close: 9, Timestamp: timestamppb.New(thursday)},
	}}
	algo, _ := test_helper.WorkerSimulator(test.T(), &test_helper.WorkerFunction{Name: "handle_data"})
	pool, err := worker.NewPool(context.TODO(), algo, worker.WithBars(source, "yahoo"), worker.WithSessionBarRequired(fixtureCalendar()))
	test.Require().NoError(err)
	defer pool.Close()

	portfolio := &finance_pb.Portfolio{Timestamp: timestamppb.New(saturday)}
	_, err = pool.Process(context.TODO(), saturday, []string{"AAPL"}, portfolio)
	test.Require().ErrorIs(err, worker.ErrMissingAsOfBar)
	test.Require().EqualError(err, missingAsOfBarMessage)
}

func (test *PoolTest) TestTradingKeepsABarNewerThanTheSession() {
	// 10:00 New York is before the close, so the as-of session is Thursday.
	// Yahoo's Friday open stamp is 14:30 UTC, which is already <= asOf.
	// That newer bar is the history, not a reason to refuse.
	asOf := time.Date(2026, time.November, 13, 15, 0, 0, 0, time.UTC)
	thursday := time.Date(2026, time.November, 12, 14, 30, 0, 0, time.UTC)
	fridayOpen := time.Date(2026, time.November, 13, 14, 30, 0, 0, time.UTC)
	source := &recordingBars{bars: []*finance_pb.OHLC{
		{Symbol: "AAPL", Close: 9, Timestamp: timestamppb.New(thursday)},
		{Symbol: "AAPL", Close: 10, Timestamp: timestamppb.New(fridayOpen)},
	}}
	var got time.Time
	handler := func(req *servicePb.WorkerRequest) *servicePb.WorkerResponse {
		for _, bar := range req.GetBars() {
			when := bar.GetTimestamp().AsTime()
			if when.After(got) {
				got = when
			}
		}
		return &servicePb.WorkerResponse{}
	}
	pool, socket := test.startBarsPool(source, "yahoo", fixtureCalendar(), handler)
	defer socket.Close()
	defer pool.Close()

	portfolio := &finance_pb.Portfolio{Timestamp: timestamppb.New(asOf)}
	_, err := pool.Process(context.TODO(), asOf, []string{"AAPL"}, portfolio)
	test.Require().NoError(err)
	test.True(fridayOpen.Equal(got))
}

func (test *PoolTest) TestTradingFailsWhenTheCalendarHasNoSession() {
	asOf := time.Date(2026, time.November, 13, 22, 0, 0, 0, time.UTC)
	source := &recordingBars{bars: []*finance_pb.OHLC{
		{Symbol: "AAPL", Close: 10, Timestamp: timestamppb.New(asOf)},
	}}
	algo, _ := test_helper.WorkerSimulator(test.T(), &test_helper.WorkerFunction{Name: "handle_data"})
	pool, err := worker.NewPool(context.TODO(), algo, worker.WithBars(source, "yahoo"), worker.WithSessionBarRequired(memoryCalendar{}))
	test.Require().NoError(err)
	defer pool.Close()

	portfolio := &finance_pb.Portfolio{Timestamp: timestamppb.New(asOf)}
	_, err = pool.Process(context.TODO(), asOf, []string{"AAPL"}, portfolio)
	test.Require().ErrorIs(err, worker.ErrNoCompletedSession)
	test.Require().Equal("Please report it at https://github.com/lhjnilsson/foreverbull/issues with this message.", usercopy.ReportIt)
	test.Require().EqualError(err, "trading: foreverbull's XNYS calendar has no finished trading session at or before 2026-11-13T22:00:00Z, so it won't trade. "+usercopy.ReportIt)
}

func (test *PoolTest) TestTradingHidesACalendarReadFailure() {
	asOf := time.Date(2026, time.November, 13, 22, 0, 0, 0, time.UTC)
	cause := errors.New("pq: password=secret relation trading_calendar_date")
	source := &recordingBars{bars: []*finance_pb.OHLC{
		{Symbol: "AAPL", Close: 10, Timestamp: timestamppb.New(asOf)},
	}}
	algo, _ := test_helper.WorkerSimulator(test.T(), &test_helper.WorkerFunction{Name: "handle_data"})
	pool, err := worker.NewPool(context.TODO(), algo,
		worker.WithBars(source, "yahoo"),
		worker.WithSessionBarRequired(failingCalendar{err: worker.CalendarReadFailed(cause)}),
	)
	test.Require().NoError(err)
	defer pool.Close()

	portfolio := &finance_pb.Portfolio{Timestamp: timestamppb.New(asOf)}
	_, err = pool.Process(context.TODO(), asOf, []string{"AAPL"}, portfolio)
	test.Require().ErrorIs(err, worker.ErrCalendarRead)
	test.Require().ErrorIs(err, cause)
	test.Require().EqualError(err, "trading: foreverbull couldn't read its XNYS trading calendar, so it won't trade. "+usercopy.RetryStep("fbull trading run")+" If it repeats, please report it at "+usercopy.IssuesURL+" with this message.")
	test.NotContains(err.Error(), "password")
	test.NotContains(err.Error(), "relation")
}

func (test *PoolTest) TestTradingNilCalendarIsNotConfigured() {
	asOf := time.Date(2026, time.November, 13, 22, 0, 0, 0, time.UTC)
	source := &recordingBars{bars: []*finance_pb.OHLC{
		{Symbol: "AAPL", Close: 10, Timestamp: timestamppb.New(asOf)},
	}}
	algo, _ := test_helper.WorkerSimulator(test.T(), &test_helper.WorkerFunction{Name: "handle_data"})
	pool, err := worker.NewPool(context.TODO(), algo, worker.WithBars(source, "yahoo"), worker.WithSessionBarRequired(nil))
	test.Require().NoError(err)
	defer pool.Close()

	portfolio := &finance_pb.Portfolio{Timestamp: timestamppb.New(asOf)}
	_, err = pool.Process(context.TODO(), asOf, []string{"AAPL"}, portfolio)
	test.Require().ErrorIs(err, worker.ErrCalendarNotConfigured)
	test.Require().NotErrorIs(err, worker.ErrNoCompletedSession)
	test.Require().EqualError(err, "trading: this broker wasn't set up with the XNYS trading calendar, so it won't trade. "+usercopy.ReportIt)
}

func (test *PoolTest) TestTradingStillNamesASymbolWithNoBars() {
	asOf, _, _ := asOfSessionFixtures()
	source := &recordingBars{bars: []*finance_pb.OHLC{
		{Symbol: "MSFT", Close: 11, Timestamp: timestamppb.New(asOf)},
	}}
	algo, _ := test_helper.WorkerSimulator(test.T(), &test_helper.WorkerFunction{Name: "handle_data"})
	pool, err := worker.NewPool(context.TODO(), algo, worker.WithBars(source, "yahoo"), worker.WithSessionBarRequired(fixtureCalendar()))
	test.Require().NoError(err)
	defer pool.Close()

	portfolio := &finance_pb.Portfolio{Timestamp: timestamppb.New(asOf)}
	_, err = pool.Process(context.TODO(), asOf, []string{"AAPL"}, portfolio)
	test.Require().EqualError(err, "no bars for AAPL on 2026-11-13")
	test.Require().NotErrorIs(err, worker.ErrMissingAsOfBar)
}

func (test *PoolTest) startBarsPool(source worker.BarSource, provider string, calendar worker.SessionCalendar, handler func(*servicePb.WorkerRequest) *servicePb.WorkerResponse) (worker.Pool, mangos.Socket) {
	test.T().Helper()
	algo, runner := test_helper.WorkerSimulator(test.T(), &test_helper.WorkerFunction{CB: handler, Name: "handle_data"})
	opts := []worker.Option{worker.WithBars(source, provider)}
	if calendar != nil {
		opts = append(opts, worker.WithSessionBarRequired(calendar))
	}
	pool, err := worker.NewPool(context.TODO(), algo, opts...)
	test.Require().NoError(err)
	configuration := pool.Configure()
	socket, err := rep.NewSocket()
	test.Require().NoError(err)
	test.Require().NoError(socket.Dial(fmt.Sprintf("tcp://localhost:%d", configuration.BrokerPort)))
	test.Require().NoError(socket.SetOption(mangos.OptionRecvDeadline, time.Second))
	test.Require().NoError(socket.SetOption(mangos.OptionSendDeadline, time.Second))
	go runner(socket)
	return pool, socket
}

func (test *PoolTest) TestProcessEmptySymbolListIsNotMissingBars() {
	source := &recordingBars{err: fmt.Errorf("list should not be called")}
	handler := func(_ *servicePb.WorkerRequest) *servicePb.WorkerResponse {
		return &servicePb.WorkerResponse{}
	}
	algo, runner := test_helper.WorkerSimulator(test.T(), &test_helper.WorkerFunction{CB: handler, Name: "handle_data"})
	pool, err := worker.NewPool(context.TODO(), algo, worker.WithBars(source, "yahoo"))
	test.Require().NoError(err)
	defer pool.Close()

	configuration := pool.Configure()
	socket, err := rep.NewSocket()
	test.Require().NoError(err)
	defer socket.Close()
	test.Require().NoError(socket.Dial(fmt.Sprintf("tcp://localhost:%d", configuration.BrokerPort)))
	test.Require().NoError(socket.SetOption(mangos.OptionRecvDeadline, time.Second))
	test.Require().NoError(socket.SetOption(mangos.OptionSendDeadline, time.Second))
	go runner(socket)

	when := time.Date(2024, time.June, 3, 0, 0, 0, 0, time.UTC)
	_, err = pool.Process(context.TODO(), when, nil, &finance_pb.Portfolio{})
	test.Require().NoError(err)
	_, err = pool.Process(context.TODO(), when, []string{}, &finance_pb.Portfolio{})
	test.Require().NoError(err)
	test.Equal(0, source.calls)
}

func (test *PoolTest) TestSimple() {
	cb := func(_ *servicePb.WorkerRequest) *servicePb.WorkerResponse {
		return &servicePb.WorkerResponse{}
	}
	functions := []*test_helper.WorkerFunction{
		{
			CB:       cb,
			Name:     "test",
			Parallel: true,
		},
	}
	algo, runner := test_helper.WorkerSimulator(test.T(), functions...)
	pool, err := worker.NewPool(context.TODO(), algo)
	test.Require().NoError(err)

	configuration := pool.Configure()

	socket, err := rep.NewSocket()
	test.Require().NoError(err)
	test.Require().NoError(socket.Dial(fmt.Sprintf("tcp://localhost:%d", configuration.BrokerPort)))
	test.Require().NoError(socket.SetOption(mangos.OptionRecvDeadline, time.Second))
	test.Require().NoError(socket.SetOption(mangos.OptionSendDeadline, time.Second))

	namespaceSocket, err := req.NewSocket()
	test.Require().NoError(err)
	test.Require().NoError(namespaceSocket.Dial(fmt.Sprintf("tcp://localhost:%d", configuration.NamespacePort)))

	go runner(socket)

	orders, err := pool.Process(context.TODO(), time.Now(), []string{"test"}, &finance_pb.Portfolio{})
	test.Require().NoError(err)
	test.Require().Empty(orders)

	socket.Close()
	namespaceSocket.Close()
	test.Require().NoError(pool.Close())
}

func (test *PoolTest) startWorker(functions ...*test_helper.WorkerFunction) (worker.Pool, mangos.Socket) {
	test.T().Helper()

	algo, runner := test_helper.WorkerSimulator(test.T(), functions...)
	pool, err := worker.NewPool(context.TODO(), algo)
	test.Require().NoError(err)

	configuration := pool.Configure()
	socket, err := rep.NewSocket()
	test.Require().NoError(err)
	test.Require().NoError(socket.Dial(fmt.Sprintf("tcp://localhost:%d", configuration.BrokerPort)))
	test.Require().NoError(socket.SetOption(mangos.OptionRecvDeadline, 5*time.Second))
	test.Require().NoError(socket.SetOption(mangos.OptionSendDeadline, 5*time.Second))
	go runner(socket)

	return pool, socket
}

func (test *PoolTest) TestProcessWorkerRequestContract() {
	var mu sync.Mutex
	var requests []*servicePb.WorkerRequest
	handler := func(req *servicePb.WorkerRequest) *servicePb.WorkerResponse {
		mu.Lock()
		defer mu.Unlock()
		requests = append(requests, proto.Clone(req).(*servicePb.WorkerRequest))
		orders := make([]*finance_pb.Order, 0, len(req.Symbols))
		for _, symbol := range req.Symbols {
			orders = append(orders, &finance_pb.Order{Symbol: symbol, Amount: 4})
		}
		return &servicePb.WorkerResponse{Task: req.Task, Orders: orders}
	}

	pool, socket := test.startWorker(
		&test_helper.WorkerFunction{CB: handler, Name: "handle_data", Parallel: false},
	)
	defer socket.Close()
	defer pool.Close()

	when := time.Date(2024, time.March, 9, 15, 4, 5, 0, time.UTC)
	portfolio := &finance_pb.Portfolio{Timestamp: timestamppb.New(when), Cash: 1000}
	orders, err := pool.Process(context.TODO(), when, []string{"AAPL", "MSFT"}, portfolio)
	test.Require().NoError(err)
	test.Require().Len(orders, 2)
	test.Equal("AAPL", orders[0].GetSymbol())
	test.Equal(int32(4), orders[0].GetAmount())
	test.Equal("MSFT", orders[1].GetSymbol())
	test.Equal(int32(4), orders[1].GetAmount())
	test.Require().Len(requests, 1)
	test.Equal("handle_data", requests[0].Task)
	test.Equal([]string{"AAPL", "MSFT"}, requests[0].Symbols)
	test.InDelta(1000.0, requests[0].Portfolio.GetCash(), 1e-9)
	test.True(when.Equal(requests[0].Portfolio.GetTimestamp().AsTime()))
}

func (test *PoolTest) TestProcessRunsFunctionsInDefinitionOrder() {
	var mu sync.Mutex
	var tasks []string
	var parallelSymbols [][]string
	handler := func(req *servicePb.WorkerRequest) *servicePb.WorkerResponse {
		mu.Lock()
		defer mu.Unlock()
		tasks = append(tasks, req.Task)
		if req.Task == "measure_assets" {
			copied := append([]string{}, req.Symbols...)
			parallelSymbols = append(parallelSymbols, copied)
		}
		return &servicePb.WorkerResponse{Task: req.Task}
	}

	pool, socket := test.startWorker(
		&test_helper.WorkerFunction{CB: handler, Name: "measure_assets", Parallel: true},
		&test_helper.WorkerFunction{CB: handler, Name: "create_orders", Parallel: false, RunLast: true},
		&test_helper.WorkerFunction{CB: handler, Name: "filter_assets", Parallel: false, RunFirst: true},
	)
	defer socket.Close()
	defer pool.Close()

	_, err := pool.Process(context.TODO(), time.Now(), []string{"AAPL", "MSFT"}, &finance_pb.Portfolio{})
	test.Require().NoError(err)
	test.Require().Len(tasks, 4)
	test.Equal("filter_assets", tasks[0])
	test.Equal("create_orders", tasks[3])
	test.ElementsMatch([]string{"measure_assets", "measure_assets"}, tasks[1:3])
	test.ElementsMatch([][]string{{"AAPL"}, {"MSFT"}}, parallelSymbols)
}

func (test *PoolTest) TestProcessReturnsTheWorkerError() {
	message := "Couldn't run backtest: the data service can't read market data. This is a bug in the Foreverbull install, not in your strategy."
	cb := func(_ *servicePb.WorkerRequest) *servicePb.WorkerResponse {
		return &servicePb.WorkerResponse{Error: &message}
	}
	pool, socket := test.startWorker(&test_helper.WorkerFunction{CB: cb, Name: "handle_data"})
	defer socket.Close()
	defer pool.Close()

	_, err := pool.Process(context.TODO(), time.Now(), []string{"AAPL"}, &finance_pb.Portfolio{})
	test.Require().EqualError(err, message)
}

func (test *PoolTest) TestProcessReturnsTheWorkerErrorFromAParallelFunction() {
	message := "Couldn't run backtest: the data service can't read market data. This is a bug in the Foreverbull install, not in your strategy."
	cb := func(_ *servicePb.WorkerRequest) *servicePb.WorkerResponse {
		return &servicePb.WorkerResponse{Error: &message}
	}
	pool, socket := test.startWorker(&test_helper.WorkerFunction{CB: cb, Name: "handle_data", Parallel: true})
	defer socket.Close()
	defer pool.Close()

	_, err := pool.Process(context.TODO(), time.Now(), []string{"AAPL", "MSFT"}, &finance_pb.Portfolio{})
	test.Require().EqualError(err, message)
}
