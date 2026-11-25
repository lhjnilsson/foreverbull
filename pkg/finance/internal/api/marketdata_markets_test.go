package api_test

import (
	"context"
	"strings"

	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/marketdata"
	internal_pb "github.com/lhjnilsson/foreverbull/pkg/pb"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (suite *MarketdataTest) londonQuotes() func() {
	return marketdata.SetQuoteReader(func(_ context.Context, _, _, symbol string) (marketdata.Quote, error) {
		switch strings.ToUpper(symbol) {
		case "VOD.L":
			return marketdata.Quote{Exchange: "LSE", Currency: "GBP", Reported: true}, nil
		case "BMW.DE":
			return marketdata.Quote{Exchange: "GER", Currency: "EUR", Reported: true}, nil
		case "XYZ":
			return marketdata.Quote{Exchange: "NMS", Currency: "CAD", Reported: true}, nil
		default:
			return marketdata.Quote{Exchange: "NMS", Currency: "USD", Reported: true}, nil
		}
	})
}

func (suite *MarketdataTest) downloadStatus(symbols []string) error {
	rsp, err := suite.client.DownloadHistoricalData(context.Background(), &servicePb.DownloadHistoricalDataRequest{
		Symbols:   symbols,
		StartDate: &internal_pb.Date{Year: 2024, Month: 1, Day: 2},
		EndDate:   &internal_pb.Date{Year: 2024, Month: 3, Day: 28},
		Provider:  "yahoo",
	})
	suite.Require().NoError(err)
	_, err = rsp.Recv()
	return err
}

func (suite *MarketdataTest) TestDownloadRefusesLondonBeforeAnyStore() {
	restore := suite.londonQuotes()
	defer restore()

	err := suite.downloadStatus([]string{"VOD.L"})
	st, ok := status.FromError(err)
	suite.Require().True(ok)
	suite.Equal(codes.FailedPrecondition, st.Code())
	suite.Require().EqualError(err, "rpc error: code = FailedPrecondition desc = VOD.L is listed outside the US (London, GBP).")

	suite.marketdata.AssertNotCalled(suite.T(), "GetAsset", mock.Anything, mock.Anything)
	suite.marketdata.AssertNotCalled(suite.T(), "GetOHLC", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	suite.marketdata.AssertNotCalled(suite.T(), "SearchAsset", mock.Anything, mock.Anything)

	var assets, bars int
	suite.Require().NoError(suite.pgx.QueryRow(context.Background(), `SELECT count(*) FROM asset`).Scan(&assets))
	suite.Require().NoError(suite.pgx.QueryRow(context.Background(), `SELECT count(*) FROM ohlc`).Scan(&bars))
	suite.Zero(assets)
	suite.Zero(bars)
}

func (suite *MarketdataTest) TestDownloadRefusesEverySymbol() {
	restore := suite.londonQuotes()
	defer restore()
	err := suite.downloadStatus([]string{"XYZ", "VOD.L"})
	suite.Require().EqualError(err, "rpc error: code = FailedPrecondition desc = 2 symbols aren't US-listed in USD: VOD.L (London, GBP), XYZ (yahoo prices it in CAD).")
	suite.marketdata.AssertNotCalled(suite.T(), "GetOHLC", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (suite *MarketdataTest) TestDownloadRefusesACurrencyWithoutASuffix() {
	restore := suite.londonQuotes()
	defer restore()
	err := suite.downloadStatus([]string{"XYZ"})
	suite.Require().EqualError(err, "rpc error: code = FailedPrecondition desc = yahoo prices XYZ in CAD, not USD.")
	suite.marketdata.AssertNotCalled(suite.T(), "GetAsset", mock.Anything, mock.Anything)
}
