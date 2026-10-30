package api_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	internalGrpc "github.com/lhjnilsson/foreverbull/internal/grpc"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/api"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/marketdata"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	internal_pb "github.com/lhjnilsson/foreverbull/pkg/pb"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type MarketdataTest struct {
	suite.Suite

	pgx        *pgxpool.Pool
	marketdata *supplier.MockMarketdata

	listener *bufconn.Listener
	server   *grpc.Server
	client   servicePb.MarketdataClient
}

func TestMarketData(t *testing.T) {
	suite.Run(t, new(MarketdataTest))
}

func (suite *MarketdataTest) SetupSuite() {
	test_helper.SetupEnvironment(suite.T(), &test_helper.Containers{
		Postgres: true,
	})
}

func (suite *MarketdataTest) TearDownSuite() {
}

func (suite *MarketdataTest) SetupTest() {
	var err error
	suite.pgx, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	suite.Require().NoError(err)
	suite.Require().NoError(repository.Recreate(context.TODO(), suite.pgx))
	suite.marketdata = supplier.NewMockMarketdata(suite.T())

	suite.listener = bufconn.Listen(1024 * 1024)
	suite.server, err = internalGrpc.NewServer()
	suite.Require().NoError(err)
	server := api.NewMarketdataServer(suite.pgx, suite.marketdata)
	servicePb.RegisterMarketdataServer(suite.server, server)

	go func() {
		suite.NoError(suite.server.Serve(suite.listener))
	}()

	resolver.SetDefaultScheme("passthrough")

	conn, err := grpc.NewClient(suite.listener.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) {
			return suite.listener.Dial()
		}),
	)
	if err != nil {
		log.Printf("error connecting to server: %v", err)
	}

	suite.client = servicePb.NewMarketdataClient(conn)
}

func (suite *MarketdataTest) TestGetAsset() {
	suite.marketdata.On("GetAsset", "AAPL").Return(&pb.Asset{Symbol: "AAPL", Name: "Apple Inc."}, nil)

	req := &servicePb.GetAssetRequest{Identifier: &servicePb.GetAssetRequest_Symbol{Symbol: "AAPL"}}
	rsp, err := suite.client.GetAsset(context.Background(), req)
	suite.Require().NoError(err)
	suite.Require().NotNil(rsp)
	suite.Equal("AAPL", rsp.Asset.Symbol)
	suite.Equal("Apple Inc.", rsp.Asset.Name)
}

func (suite *MarketdataTest) TestGetIndex() {
	suite.marketdata.On("GetIndex", "^GDAXI").Return([]*pb.Asset{
		{Symbol: "ADS.DE", Name: "Adidas AG"},
	}, nil)

	req := &servicePb.GetIndexRequest{Symbol: "^GDAXI"}
	rsp, err := suite.client.GetIndex(context.Background(), req)
	suite.Require().NoError(err)
	suite.Require().NotNil(rsp)
	suite.NotEmpty(rsp.Assets)
}

func (suite *MarketdataTest) TestDownloadHistoricalData() {
	suite.marketdata.On("GetAsset", "AAPL").Return(&pb.Asset{Symbol: "AAPL", Name: "Apple Inc."}, nil)
	suite.marketdata.On("GetAsset", "NOCORP").Return(nil, errors.New("not found"))
	suite.marketdata.On("GetOHLC", "AAPL", mock.Anything, mock.Anything).Return([]*pb.OHLC{
		{
			Timestamp: timestamppb.Now(),
			Open:      1,
			High:      2,
			Low:       0.5,
			Close:     1.5,
			Volume:    100,
		},
	}, nil)

	req := &servicePb.DownloadHistoricalDataRequest{
		Symbols:   []string{"AAPL", "NOCORP"},
		StartDate: &internal_pb.Date{Year: 2020, Month: 1, Day: 1},
		EndDate:   &internal_pb.Date{Year: 2024, Month: 06, Day: 30},
		// yahoo routes GetOHLC to the injected mock.
		Provider: "yahoo",
	}
	rsp, err := suite.client.DownloadHistoricalData(context.Background(), req)
	suite.Require().NoError(err)

	result := make(map[string]servicePb.DownloadHistoricalDataResponse_Status)
	for {
		msg, err := rsp.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			suite.Require().NoError(err)
			break
		}
		suite.Require().NoError(err)
		result[msg.GetSymbol()] = msg.GetStatus()
	}
	suite.Equal(servicePb.DownloadHistoricalDataResponse_STATUS_OK, result["AAPL"])
	suite.Equal(servicePb.DownloadHistoricalDataResponse_STATUS_ERROR, result["NOCORP"])
}

// A refused bar fails the symbol once and stores none of its bars, so the
// shared price cache never keeps a gap.
func (suite *MarketdataTest) TestDownloadHistoricalDataStoresNothingWhenABarIsRefused() {
	suite.marketdata.On("GetAsset", "XYZ").Return(&pb.Asset{Symbol: "XYZ", Name: "XYZ Corp"}, nil)
	suite.marketdata.On("GetOHLC", "XYZ", mock.Anything, mock.Anything).Return([]*pb.OHLC{
		{Timestamp: timestamppb.New(mustTime("2026-05-01")), Open: 10, High: 11, Low: 9, Close: 10.5, Volume: 100},
		{Timestamp: timestamppb.New(mustTime("2026-05-02")), Open: 0, High: 11, Low: 9, Close: 10.5, Volume: 100},
		{Timestamp: timestamppb.New(mustTime("2026-05-03")), Open: 10, High: 11, Low: 9, Close: 10.5, Volume: 100},
	}, nil)

	rsp, err := suite.client.DownloadHistoricalData(context.Background(), &servicePb.DownloadHistoricalDataRequest{
		Symbols:   []string{"XYZ"},
		StartDate: &internal_pb.Date{Year: 2026, Month: 5, Day: 1},
		EndDate:   &internal_pb.Date{Year: 2026, Month: 5, Day: 3},
		Provider:  "yahoo",
	})
	suite.Require().NoError(err)

	var msgs []*servicePb.DownloadHistoricalDataResponse
	for {
		msg, err := rsp.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		suite.Require().NoError(err)
		msgs = append(msgs, msg)
	}
	suite.Require().Len(msgs, 1, "the symbol fails once")
	suite.Equal(servicePb.DownloadHistoricalDataResponse_STATUS_ERROR, msgs[0].GetStatus())
	suite.Equal("yahoo sent an invalid price for XYZ on 2026-05-02 (0, negative or not a number)", msgs[0].GetError())

	var rows int
	suite.Require().NoError(suite.pgx.QueryRow(context.Background(),
		`SELECT count(*) FROM ohlc WHERE symbol = 'XYZ'`).Scan(&rows))
	suite.Zero(rows, "no bar of a refused batch may be stored")
}

func (suite *MarketdataTest) recvAll(stream servicePb.Marketdata_DownloadHistoricalDataClient) []*servicePb.DownloadHistoricalDataResponse {
	var msgs []*servicePb.DownloadHistoricalDataResponse
	for {
		msg, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return msgs
		}
		suite.Require().NoError(err)
		msgs = append(msgs, msg)
	}
}

// A DB error on one row rolls the whole symbol back, and its text (which may
// carry a DSN or SQL) stays in the log, never in the response.
func (suite *MarketdataTest) TestDownloadHistoricalDataScrubsStoreErrorAndRollsBack() {
	const leak = "dial postgres://admin:hunter2@10.0.0.5:5432/prod: INSERT INTO ohlc"
	_, err := suite.pgx.Exec(context.Background(), `
CREATE OR REPLACE FUNCTION fail_on_bad_row() RETURNS trigger AS $$
BEGIN
	IF NEW.close = 13.37 THEN
		RAISE EXCEPTION '`+leak+`';
	END IF;
	RETURN NEW;
END $$ LANGUAGE plpgsql;
CREATE TRIGGER fail_on_bad_row BEFORE INSERT ON ohlc FOR EACH ROW EXECUTE FUNCTION fail_on_bad_row();`)
	suite.Require().NoError(err)

	suite.marketdata.On("GetAsset", "XYZ").Return(&pb.Asset{Symbol: "XYZ", Name: "XYZ Corp"}, nil)
	suite.marketdata.On("GetAsset", "LEAK").Return(nil, errors.New(leak))
	suite.marketdata.On("GetOHLC", "XYZ", mock.Anything, mock.Anything).Return([]*pb.OHLC{
		{Timestamp: timestamppb.New(mustTime("2026-05-01")), Open: 10, High: 14, Low: 9, Close: 10.5, Volume: 100},
		{Timestamp: timestamppb.New(mustTime("2026-05-02")), Open: 10, High: 14, Low: 9, Close: 13.37, Volume: 100},
		{Timestamp: timestamppb.New(mustTime("2026-05-03")), Open: 10, High: 14, Low: 9, Close: 10.5, Volume: 100},
	}, nil)

	rsp, err := suite.client.DownloadHistoricalData(context.Background(), &servicePb.DownloadHistoricalDataRequest{
		Symbols:   []string{"XYZ", "LEAK"},
		StartDate: &internal_pb.Date{Year: 2026, Month: 5, Day: 1},
		EndDate:   &internal_pb.Date{Year: 2026, Month: 5, Day: 3},
		Provider:  "yahoo",
	})
	suite.Require().NoError(err)

	errs := make(map[string]string)
	for _, msg := range suite.recvAll(rsp) {
		suite.Equal(servicePb.DownloadHistoricalDataResponse_STATUS_ERROR, msg.GetStatus(), msg.GetSymbol())
		errs[msg.GetSymbol()] = msg.GetError()
	}
	suite.Equal("couldn't download prices for XYZ. Try again in a few minutes, or use another provider with --provider.", errs["XYZ"])
	suite.Equal("couldn't download prices for LEAK. Try again in a few minutes, or use another provider with --provider.", errs["LEAK"])
	for symbol, text := range errs {
		suite.NotContains(text, "hunter2", symbol)
		suite.NotContains(text, "INSERT", symbol)
	}

	var rows int
	suite.Require().NoError(suite.pgx.QueryRow(context.Background(),
		`SELECT count(*) FROM ohlc WHERE symbol = 'XYZ'`).Scan(&rows))
	suite.Zero(rows, "a DB error on one row must leave none of the symbol's bars")
}

// Many workers report on one stream; run with -race to catch concurrent Send.
func (suite *MarketdataTest) TestDownloadHistoricalDataManySymbols() {
	const n = 60
	symbols := make([]string, 0, n)
	for i := range n {
		symbols = append(symbols, fmt.Sprintf("SYM%02d", i))
	}
	suite.marketdata.On("GetAsset", mock.Anything).Return(func(symbol string) (*pb.Asset, error) {
		return &pb.Asset{Symbol: symbol, Name: symbol}, nil
	})
	suite.marketdata.On("GetOHLC", mock.Anything, mock.Anything, mock.Anything).Return([]*pb.OHLC{
		{Timestamp: timestamppb.New(mustTime("2026-05-01")), Open: 10, High: 11, Low: 9, Close: 10.5, Volume: 100},
	}, nil)

	rsp, err := suite.client.DownloadHistoricalData(context.Background(), &servicePb.DownloadHistoricalDataRequest{
		Symbols:   symbols,
		StartDate: &internal_pb.Date{Year: 2026, Month: 5, Day: 1},
		EndDate:   &internal_pb.Date{Year: 2026, Month: 5, Day: 1},
		Provider:  "yahoo",
	})
	suite.Require().NoError(err)

	seen := make(map[string]bool)
	for _, msg := range suite.recvAll(rsp) {
		suite.Equal(servicePb.DownloadHistoricalDataResponse_STATUS_OK, msg.GetStatus(), msg.GetSymbol())
		seen[msg.GetSymbol()] = true
	}
	suite.Len(seen, n)
}

func (suite *MarketdataTest) TestDownloadAssetsNoEndDate() {
	suite.marketdata.On("GetAsset", "AAPL").Return(&pb.Asset{Symbol: "AAPL", Name: "Apple Inc."}, nil)
	suite.marketdata.On("GetAsset", "NOCORP").Return(nil, errors.New("not found"))
	suite.marketdata.On("GetOHLC", "AAPL", mock.Anything, mock.Anything).Return([]*pb.OHLC{
		{
			Timestamp: timestamppb.Now(),
			Open:      1,
			High:      2,
			Low:       0.5,
			Close:     1.5,
			Volume:    100,
		},
	}, nil)

	req := &servicePb.DownloadHistoricalDataRequest{
		Symbols:   []string{"AAPL", "NOCORP"},
		StartDate: &internal_pb.Date{Year: 2020, Month: 1, Day: 1},
		// yahoo routes GetOHLC to the injected mock.
		Provider: "yahoo",
	}
	rsp, err := suite.client.DownloadHistoricalData(context.Background(), req)
	suite.Require().NoError(err)

	result := make(map[string]servicePb.DownloadHistoricalDataResponse_Status)
	for {
		msg, err := rsp.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			suite.Require().NoError(err)
			break
		}
		suite.Require().NoError(err)
		result[msg.GetSymbol()] = msg.GetStatus()
	}
	suite.Equal(servicePb.DownloadHistoricalDataResponse_STATUS_OK, result["AAPL"])
	suite.Equal(servicePb.DownloadHistoricalDataResponse_STATUS_ERROR, result["NOCORP"])
}

func (suite *MarketdataTest) TestDownloadHistoricalDataStopsOnBlockedProvider() {
	symbols := []string{"AAPL", "MSFT", "NVDA", "AMZN", "GOOG", "META", "TSLA", "AVGO", "COST", "NFLX", "AMD", "PEP"}
	for _, symbol := range symbols {
		suite.marketdata.On("GetAsset", symbol).Return(&pb.Asset{Symbol: symbol, Name: symbol}, nil).Maybe()
		suite.marketdata.On("GetOHLC", symbol, mock.Anything, mock.Anything).
			Return(nil, &marketdata.BlockedError{Provider: "yahoo", Symbol: symbol}).Maybe()
	}

	req := &servicePb.DownloadHistoricalDataRequest{
		Symbols:   symbols,
		StartDate: &internal_pb.Date{Year: 2020, Month: 1, Day: 1},
		// yahoo routes GetOHLC to the injected mock.
		Provider: "yahoo",
	}
	rsp, err := suite.client.DownloadHistoricalData(context.Background(), req)
	suite.Require().NoError(err)

	var streamErr error
	responses := 0
	for {
		msg, err := rsp.Recv()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				streamErr = err
			}
			break
		}
		responses++
		suite.NotEqual(servicePb.DownloadHistoricalDataResponse_STATUS_ERROR, msg.GetStatus(),
			"%s was reported as failed, which disables it", msg.GetSymbol())
	}
	suite.Zero(responses, "a blocked symbol must not be reported per symbol")

	st, ok := status.FromError(streamErr)
	suite.Require().True(ok, "want a gRPC status, got %v", streamErr)
	suite.Equal(codes.FailedPrecondition, st.Code())
	suite.Regexp(`^Yahoo blocked the download for [A-Z]+ \(it returned a bot-check page instead of data\)\. Retry with --provider stooq\.$`, st.Message())
}

const metaISIN = "US30303M1027"

// Yahoo has no bars under FB. The ISIN's current ticker (META) still has the
// history, including dates before the rename. Download must fetch under META,
// store those bars, record META as a later alias, and report OK for FB.
func (suite *MarketdataTest) TestDownloadHistoricalDataUsesLaterTickerWhenAskedHasNoBars() {
	ctx := context.Background()
	instruments := repository.Instruments{Conn: suite.pgx}
	_, err := instruments.Store(ctx, metaISIN, "Facebook Inc")
	suite.Require().NoError(err)
	_, err = suite.pgx.Exec(ctx, `
		INSERT INTO asset_symbol (isin, symbol, from_date, to_date)
		VALUES ($1, 'FB', '2019-01-01'::date, NULL)`, metaISIN)
	suite.Require().NoError(err)

	suite.marketdata.On("GetAsset", "FB").Return(&pb.Asset{Symbol: "FB", Name: "ProShares Buffer ETF"}, nil)
	suite.marketdata.On("GetAsset", "META").Return(&pb.Asset{Symbol: "META", Name: "Meta Platforms, Inc.", ISIN: metaISIN}, nil)
	suite.marketdata.On("GetOHLC", "FB", mock.Anything, mock.Anything).
		Return(nil, yahooNotFound("FB"))
	suite.marketdata.On("GetOHLC", "META", mock.Anything, mock.Anything).Return([]*pb.OHLC{
		{
			Timestamp: timestamppb.New(mustTime("2019-06-03")),
			Open:      10, High: 11, Low: 9, Close: 10.5, Volume: 1000,
		},
	}, nil)
	suite.marketdata.On("SearchAsset", metaISIN).Return([]*pb.Asset{
		{Symbol: "META", Name: "Meta Platforms, Inc.", ISIN: metaISIN},
	}, nil)

	req := &servicePb.DownloadHistoricalDataRequest{
		Symbols:   []string{"FB"},
		StartDate: &internal_pb.Date{Year: 2019, Month: 1, Day: 1},
		EndDate:   &internal_pb.Date{Year: 2020, Month: 12, Day: 31},
		Provider:  "yahoo",
	}
	rsp, err := suite.client.DownloadHistoricalData(context.Background(), req)
	suite.Require().NoError(err)

	var statuses []servicePb.DownloadHistoricalDataResponse_Status
	for {
		msg, err := rsp.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		suite.Require().NoError(err)
		suite.Equal("FB", msg.GetSymbol())
		statuses = append(statuses, msg.GetStatus())
	}
	suite.Equal([]servicePb.DownloadHistoricalDataResponse_Status{
		servicePb.DownloadHistoricalDataResponse_STATUS_OK,
	}, statuses)

	var metaBars, fbBars int
	suite.Require().NoError(suite.pgx.QueryRow(ctx,
		`SELECT count(*) FROM ohlc WHERE provider='yahoo' AND symbol='META'`).Scan(&metaBars))
	suite.Require().NoError(suite.pgx.QueryRow(ctx,
		`SELECT count(*) FROM ohlc WHERE provider='yahoo' AND symbol='FB'`).Scan(&fbBars))
	suite.Equal(1, metaBars, "bars must be stored under META")
	suite.Zero(fbBars, "Yahoo serves no FB bars")

	aliases, err := instruments.ListAliases(ctx, metaISIN)
	suite.Require().NoError(err)
	symbols := make([]string, 0, len(aliases))
	for _, a := range aliases {
		symbols = append(symbols, a.Symbol)
	}
	suite.Contains(symbols, "META")
	suite.Contains(symbols, "FB")
}

func (suite *MarketdataTest) TestDownloadHistoricalDataFailsWhenNoTickerHasBars() {
	ctx := context.Background()
	instruments := repository.Instruments{Conn: suite.pgx}
	_, err := instruments.Store(ctx, metaISIN, "Facebook Inc")
	suite.Require().NoError(err)
	_, err = suite.pgx.Exec(ctx, `
		INSERT INTO asset_symbol (isin, symbol, from_date, to_date)
		VALUES ($1, 'FB', '2019-01-01'::date, NULL)`, metaISIN)
	suite.Require().NoError(err)

	suite.marketdata.On("GetAsset", "FB").Return(&pb.Asset{Symbol: "FB", Name: "Facebook"}, nil)
	suite.marketdata.On("GetOHLC", "FB", mock.Anything, mock.Anything).
		Return(nil, yahooNotFound("FB"))
	suite.marketdata.On("GetOHLC", "META", mock.Anything, mock.Anything).
		Return(nil, yahooNotFound("META")).Maybe()
	suite.marketdata.On("SearchAsset", metaISIN).Return([]*pb.Asset{
		{Symbol: "META", Name: "Meta Platforms, Inc.", ISIN: metaISIN},
	}, nil)

	req := &servicePb.DownloadHistoricalDataRequest{
		Symbols:   []string{"FB"},
		StartDate: &internal_pb.Date{Year: 2019, Month: 1, Day: 1},
		EndDate:   &internal_pb.Date{Year: 2020, Month: 12, Day: 31},
		Provider:  "yahoo",
	}
	rsp, err := suite.client.DownloadHistoricalData(context.Background(), req)
	suite.Require().NoError(err)

	msg, err := rsp.Recv()
	suite.Require().NoError(err)
	suite.Equal("FB", msg.GetSymbol())
	suite.Equal(servicePb.DownloadHistoricalDataResponse_STATUS_ERROR, msg.GetStatus())
	suite.True(strings.HasPrefix(msg.GetError(), "no price data for "), msg.GetError())
}

// yahooNotFound is Yahoo's answer for a ticker it has no chart for. It is
// an answer, not a failure, so the download may try another ticker or
// report no price data.
func yahooNotFound(symbol string) error {
	return fmt.Errorf("fail to get OHLC data for symbol %s: No data found, symbol may be delisted: %w",
		symbol, marketdata.ErrNoBars)
}

// A GetOHLC failure (5xx, network, timeout) says nothing about whether the
// symbol has prices. It must be reported with its own text, not as "no
// price data", so a symbol-list create fails instead of leaving it out. No
// other ticker is tried.
func (suite *MarketdataTest) TestDownloadHistoricalDataReportsGetOHLCFailure() {
	suite.marketdata.On("GetAsset", "AAPL").Return(&pb.Asset{Symbol: "AAPL", Name: "Apple Inc."}, nil)
	suite.marketdata.On("GetOHLC", "AAPL", mock.Anything, mock.Anything).
		Return(nil, errors.New("fail to get OHLC data for symbol AAPL: error making request: context deadline exceeded"))

	req := &servicePb.DownloadHistoricalDataRequest{
		Symbols:   []string{"AAPL"},
		StartDate: &internal_pb.Date{Year: 2019, Month: 1, Day: 1},
		EndDate:   &internal_pb.Date{Year: 2020, Month: 12, Day: 31},
		Provider:  "yahoo",
	}
	rsp, err := suite.client.DownloadHistoricalData(context.Background(), req)
	suite.Require().NoError(err)

	msg, err := rsp.Recv()
	suite.Require().NoError(err)
	suite.Equal("AAPL", msg.GetSymbol())
	suite.Equal(servicePb.DownloadHistoricalDataResponse_STATUS_ERROR, msg.GetStatus())
	suite.False(strings.HasPrefix(msg.GetError(), "no price data for "), msg.GetError())
	suite.Contains(msg.GetError(), "context deadline exceeded")
	_, err = rsp.Recv()
	suite.Require().ErrorIs(err, io.EOF)
	suite.marketdata.AssertNotCalled(suite.T(), "SearchAsset", mock.Anything)
}

// FB has no bars, and fetching under its later ticker META fails. We don't
// know whether META has prices, so the failure is reported, not "no price
// data".
func (suite *MarketdataTest) TestDownloadHistoricalDataReportsLaterTickerGetOHLCFailure() {
	ctx := context.Background()
	instruments := repository.Instruments{Conn: suite.pgx}
	_, err := instruments.Store(ctx, metaISIN, "Facebook Inc")
	suite.Require().NoError(err)
	_, err = suite.pgx.Exec(ctx, `
		INSERT INTO asset_symbol (isin, symbol, from_date, to_date)
		VALUES ($1, 'FB', '2019-01-01'::date, NULL)`, metaISIN)
	suite.Require().NoError(err)

	suite.marketdata.On("GetAsset", "FB").Return(&pb.Asset{Symbol: "FB", Name: "Facebook"}, nil)
	suite.marketdata.On("GetOHLC", "FB", mock.Anything, mock.Anything).Return(nil, yahooNotFound("FB"))
	suite.marketdata.On("GetOHLC", "META", mock.Anything, mock.Anything).
		Return(nil, errors.New("fail to get OHLC data for symbol META: error decoding response: 503 Service Unavailable"))
	suite.marketdata.On("SearchAsset", metaISIN).Return([]*pb.Asset{
		{Symbol: "META", Name: "Meta Platforms, Inc.", ISIN: metaISIN},
	}, nil)

	req := &servicePb.DownloadHistoricalDataRequest{
		Symbols:   []string{"FB"},
		StartDate: &internal_pb.Date{Year: 2019, Month: 1, Day: 1},
		EndDate:   &internal_pb.Date{Year: 2020, Month: 12, Day: 31},
		Provider:  "yahoo",
	}
	rsp, err := suite.client.DownloadHistoricalData(ctx, req)
	suite.Require().NoError(err)

	msg, err := rsp.Recv()
	suite.Require().NoError(err)
	suite.Equal("FB", msg.GetSymbol())
	suite.Equal(servicePb.DownloadHistoricalDataResponse_STATUS_ERROR, msg.GetStatus())
	suite.False(strings.HasPrefix(msg.GetError(), "no price data for "), msg.GetError())
	suite.Contains(msg.GetError(), "503 Service Unavailable")
}

// Yahoo sometimes answers a listed symbol (BK) with no data. Download must
// ask once more and store the bars from the second answer.
func (suite *MarketdataTest) TestDownloadHistoricalDataRetriesAnEmptyAnswerOnce() {
	ctx := context.Background()
	suite.marketdata.On("GetAsset", "BK").Return(&pb.Asset{Symbol: "BK", Name: "The Bank of New York Mellon Corporation"}, nil)
	suite.marketdata.On("GetOHLC", "BK", mock.Anything, mock.Anything).Return([]*pb.OHLC{}, nil).Once()
	suite.marketdata.On("GetOHLC", "BK", mock.Anything, mock.Anything).Return([]*pb.OHLC{
		{
			Timestamp: timestamppb.New(mustTime("2024-01-02")),
			Open:      51.5, High: 52.4, Low: 51.2, Close: 52.1, Volume: 4200000,
		},
	}, nil).Once()

	req := &servicePb.DownloadHistoricalDataRequest{
		Symbols:   []string{"BK"},
		StartDate: &internal_pb.Date{Year: 2024, Month: 1, Day: 2},
		EndDate:   &internal_pb.Date{Year: 2024, Month: 3, Day: 28},
		Provider:  "yahoo",
	}
	rsp, err := suite.client.DownloadHistoricalData(ctx, req)
	suite.Require().NoError(err)

	msg, err := rsp.Recv()
	suite.Require().NoError(err)
	suite.Equal("BK", msg.GetSymbol())
	suite.Equal(servicePb.DownloadHistoricalDataResponse_STATUS_OK, msg.GetStatus(), msg.GetError())

	var bars int
	suite.Require().NoError(suite.pgx.QueryRow(ctx,
		`SELECT count(*) FROM ohlc WHERE provider='yahoo' AND symbol='BK'`).Scan(&bars))
	suite.Equal(1, bars)
	suite.marketdata.AssertNumberOfCalls(suite.T(), "GetOHLC", 2)
}

// A London listing is stored under the symbol the user asked for. Stooq
// fetches VOD.L as vod.uk, and a bar labelled with the provider's ticker must
// not end up stored under that ticker.
func (suite *MarketdataTest) TestDownloadHistoricalDataStoresAskedSymbolNotProviderTicker() {
	ctx := context.Background()
	suite.marketdata.On("GetAsset", "VOD.L").Return(&pb.Asset{Symbol: "VOD.L", Name: "Vodafone Group Plc"}, nil)
	suite.marketdata.On("GetOHLC", "VOD.L", mock.Anything, mock.Anything).Return([]*pb.OHLC{
		{
			Symbol:    "vod.uk",
			Timestamp: timestamppb.New(mustTime("2024-01-02")),
			Open:      69.1, High: 70.2, Low: 68.9, Close: 69.8, Volume: 51000000,
		},
	}, nil)

	req := &servicePb.DownloadHistoricalDataRequest{
		Symbols:   []string{"VOD.L"},
		StartDate: &internal_pb.Date{Year: 2024, Month: 1, Day: 2},
		EndDate:   &internal_pb.Date{Year: 2024, Month: 1, Day: 3},
		Provider:  "yahoo",
	}
	rsp, err := suite.client.DownloadHistoricalData(ctx, req)
	suite.Require().NoError(err)
	msg, err := rsp.Recv()
	suite.Require().NoError(err)
	suite.Equal(servicePb.DownloadHistoricalDataResponse_STATUS_OK, msg.GetStatus(), msg.GetError())

	var symbols []string
	rows, err := suite.pgx.Query(ctx, `SELECT DISTINCT symbol FROM ohlc WHERE provider='yahoo'`)
	suite.Require().NoError(err)
	for rows.Next() {
		var symbol string
		suite.Require().NoError(rows.Scan(&symbol))
		symbols = append(symbols, symbol)
	}
	suite.Require().NoError(rows.Err())
	suite.Equal([]string{"VOD.L"}, symbols)
}

// FB gets Yahoo's Not Found and the search for the instrument's current
// ticker fails. The search may have missed a ticker with bars, so the
// download reports that failure, not "no price data".
func (suite *MarketdataTest) TestDownloadHistoricalDataReportsSearchFailureWhenNoTickerHasBars() {
	ctx := context.Background()
	instruments := repository.Instruments{Conn: suite.pgx}
	_, err := instruments.Store(ctx, metaISIN, "Facebook Inc")
	suite.Require().NoError(err)
	_, err = suite.pgx.Exec(ctx, `
		INSERT INTO asset_symbol (isin, symbol, from_date, to_date)
		VALUES ($1, 'FB', '2019-01-01'::date, NULL)`, metaISIN)
	suite.Require().NoError(err)

	suite.marketdata.On("GetAsset", "FB").Return(&pb.Asset{Symbol: "FB", Name: "Facebook"}, nil)
	suite.marketdata.On("GetOHLC", "FB", mock.Anything, mock.Anything).Return(nil, yahooNotFound("FB"))
	suite.marketdata.On("SearchAsset", metaISIN).Return(nil, errors.New("fail to search asset: 503 Service Unavailable"))

	req := &servicePb.DownloadHistoricalDataRequest{
		Symbols:   []string{"FB"},
		StartDate: &internal_pb.Date{Year: 2019, Month: 1, Day: 1},
		EndDate:   &internal_pb.Date{Year: 2020, Month: 12, Day: 31},
		Provider:  "yahoo",
	}
	rsp, err := suite.client.DownloadHistoricalData(ctx, req)
	suite.Require().NoError(err)

	msg, err := rsp.Recv()
	suite.Require().NoError(err)
	suite.Equal("FB", msg.GetSymbol())
	suite.Equal(servicePb.DownloadHistoricalDataResponse_STATUS_ERROR, msg.GetStatus())
	suite.False(strings.HasPrefix(msg.GetError(), "no price data for "), msg.GetError())
	suite.Contains(msg.GetError(), "503 Service Unavailable")
	_, err = rsp.Recv()
	suite.Require().ErrorIs(err, io.EOF)
}

// The search fails, but META is already an asset_symbol alias of the
// instrument and has bars, so they are used and FB downloads OK.
func (suite *MarketdataTest) TestDownloadHistoricalDataUsesAliasWithBarsWhenSearchFails() {
	ctx := context.Background()
	instruments := repository.Instruments{Conn: suite.pgx}
	_, err := instruments.Store(ctx, metaISIN, "Facebook Inc")
	suite.Require().NoError(err)
	_, err = suite.pgx.Exec(ctx, `
		INSERT INTO asset_symbol (isin, symbol, from_date, to_date)
		VALUES ($1, 'FB', '2019-01-01'::date, '2022-06-08'::date),
		       ($1, 'META', '2022-06-09'::date, NULL)`, metaISIN)
	suite.Require().NoError(err)

	suite.marketdata.On("GetAsset", "FB").Return(&pb.Asset{Symbol: "FB", Name: "Facebook"}, nil)
	suite.marketdata.On("GetAsset", "META").Return(&pb.Asset{Symbol: "META", Name: "Meta Platforms, Inc.", ISIN: metaISIN}, nil)
	suite.marketdata.On("GetOHLC", "FB", mock.Anything, mock.Anything).Return(nil, yahooNotFound("FB"))
	suite.marketdata.On("GetOHLC", "META", mock.Anything, mock.Anything).Return([]*pb.OHLC{
		{
			Timestamp: timestamppb.New(mustTime("2019-06-03")),
			Open:      10, High: 11, Low: 9, Close: 10.5, Volume: 1000,
		},
	}, nil)
	suite.marketdata.On("SearchAsset", metaISIN).Return(nil, errors.New("fail to search asset: 503 Service Unavailable"))

	req := &servicePb.DownloadHistoricalDataRequest{
		Symbols:   []string{"FB"},
		StartDate: &internal_pb.Date{Year: 2019, Month: 1, Day: 1},
		EndDate:   &internal_pb.Date{Year: 2020, Month: 12, Day: 31},
		Provider:  "yahoo",
	}
	rsp, err := suite.client.DownloadHistoricalData(ctx, req)
	suite.Require().NoError(err)

	msg, err := rsp.Recv()
	suite.Require().NoError(err)
	suite.Equal("FB", msg.GetSymbol())
	suite.Equal(servicePb.DownloadHistoricalDataResponse_STATUS_OK, msg.GetStatus(), msg.GetError())

	var metaBars int
	suite.Require().NoError(suite.pgx.QueryRow(ctx,
		`SELECT count(*) FROM ohlc WHERE provider='yahoo' AND symbol='META'`).Scan(&metaBars))
	suite.Equal(1, metaBars, "bars must be stored under META")
}

func mustTime(day string) time.Time {
	t, err := time.Parse("2006-01-02", day)
	if err != nil {
		panic(err)
	}
	return t
}

func (suite *MarketdataTest) TestGetOHLCReadsStoredBars() {
	ctx := context.Background()
	assetRepo := repository.Asset{Conn: suite.pgx}
	suite.Require().NoError(assetRepo.Store(ctx, "AAPL", "Apple"))
	ohlcRepo := repository.OHLC{Conn: suite.pgx}
	day := time.Date(2021, 3, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		suite.Require().NoError(ohlcRepo.Store(ctx, "yahoo", "AAPL", day.AddDate(0, 0, i), 1, 2, 0.5, float64(100+i), 10))
	}

	rsp, err := suite.client.GetOHLC(ctx, &servicePb.GetOHLCRequest{
		Symbols: []string{"AAPL"},
		AsOf:    timestamppb.New(day.AddDate(0, 0, 2)),
		Limit:   2,
	})
	suite.Require().NoError(err)
	suite.Require().Len(rsp.Bars, 2)
	suite.InDelta(101, rsp.Bars[0].Close, 0.0001)
	suite.InDelta(102, rsp.Bars[1].Close, 0.0001)
}

// GetOHLC for BRK.B reads the bars stored as BRKB.
func (suite *MarketdataTest) TestGetOHLCReadsShareClassUnderStoredName() {
	ctx := context.Background()
	assetRepo := repository.Asset{Conn: suite.pgx}
	suite.Require().NoError(assetRepo.Store(ctx, "BRKB", "Berkshire Hathaway B"))
	ohlcRepo := repository.OHLC{Conn: suite.pgx}
	day := time.Date(2021, 3, 1, 0, 0, 0, 0, time.UTC)
	suite.Require().NoError(ohlcRepo.Store(ctx, "yahoo", "BRKB", day, 1, 2, 0.5, 250, 10))

	rsp, err := suite.client.GetOHLC(ctx, &servicePb.GetOHLCRequest{
		Symbols: []string{"BRK.B"},
		AsOf:    timestamppb.New(day),
	})
	suite.Require().NoError(err)
	suite.Require().Len(rsp.Bars, 1)
	suite.Equal("BRKB", rsp.Bars[0].GetSymbol())
	suite.InDelta(250, rsp.Bars[0].GetClose(), 0.0001)
}

func (suite *MarketdataTest) TestGetOHLCRejectsEmptySymbols() {
	_, err := suite.client.GetOHLC(context.Background(), &servicePb.GetOHLCRequest{
		AsOf: timestamppb.Now(),
	})
	suite.Require().Error(err)
	st, ok := status.FromError(err)
	suite.Require().True(ok)
	suite.Equal(codes.InvalidArgument, st.Code())
}
