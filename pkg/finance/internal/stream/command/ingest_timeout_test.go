package command_test

import (
	"context"
	"fmt"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/stream/command"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	"github.com/stretchr/testify/mock"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func may2026Bar() []*pb.OHLC {
	return []*pb.OHLC{{
		Timestamp: timestamppb.New(time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)),
		Open:      10, High: 11, Low: 9, Close: 10.5, Volume: 100,
	}}
}

// The ingest's time limit stops the work at the symbol whose provider never
// answers. The task state names it, the symbol before it stays stored, and
// the symbol after it is never asked for.
func (test *IngestCommandTest) TestIngestStopsAtItsTimeLimit() {
	defer command.SetIngestTimeLimit(time.Second)()
	md := supplier.NewMockMarketdata(test.T())
	for _, symbol := range []string{"AAPL", "SLOW"} {
		md.On("GetAsset", mock.Anything, symbol).Return(&pb.Asset{Symbol: symbol, Name: symbol}, nil)
	}
	md.On("GetOHLC", mock.Anything, "AAPL", mock.Anything, mock.Anything).Return(may2026Bar(), nil)
	md.On("GetOHLC", mock.Anything, "SLOW", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			callCtx, _ := args.Get(0).(context.Context)
			<-callCtx.Done()
		}).Return(nil, context.DeadlineExceeded)

	began := time.Now()
	err := test.ingestSymbols(md, "AAPL", "SLOW", "LATER")
	test.Less(time.Since(began), 10*time.Second, "the limit must end the ingest")
	test.Require().EqualError(err, "yahoo took too long to respond for SLOW, so the price import stopped. "+
		"Prices imported before it were kept. Try again later, or use another provider with --provider.")

	kept, err := test.ohlc.List(context.Background(), "yahoo", []string{"AAPL"}, time.Now(), 0)
	test.Require().NoError(err)
	test.Len(kept, 1, "a symbol finished before the limit stays stored")
	md.AssertNotCalled(test.T(), "GetAsset", mock.Anything, "LATER")
}

// A provider call that times out while the ingest still has time stops the
// ingest with the timeout sentence; the asset lookup is always Yahoo.
func (test *IngestCommandTest) TestIngestReportsAProviderTimeout() {
	md := supplier.NewMockMarketdata(test.T())
	md.On("GetAsset", mock.Anything, "AAPL").Return(nil, fmt.Errorf("error: %w", context.DeadlineExceeded))

	err := test.ingestSymbols(md, "AAPL")
	test.Require().EqualError(err, "yahoo took too long to respond for AAPL, so the price import stopped. "+
		"Prices imported before it were kept. Try again later, or use another provider with --provider.")
}
