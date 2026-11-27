package command

import (
	"context"
	"testing"

	"github.com/lhjnilsson/foreverbull/pkg/finance"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	financeServicePB "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

// isinOnlyClient answers GetAsset by ISIN and nothing else.
type isinOnlyClient struct {
	financeServicePB.MarketdataClient
	isin    string
	symbol  string
	gotISIN string
}

func (c *isinOnlyClient) GetAsset(_ context.Context, req *financeServicePB.GetAssetRequest, _ ...grpc.CallOption) (*financeServicePB.GetAssetResponse, error) {
	c.gotISIN = req.GetIsin()
	return &financeServicePB.GetAssetResponse{
		Asset: &pb.Asset{Symbol: c.symbol, ISIN: c.isin},
	}, nil
}

// First contact for an iShares holding: BRKB and its ISIN, no listings file
// and no asset_symbol row. The provider's symbol teaches the class.
func TestTeachJoinedShareClassFromHoldingsISIN(t *testing.T) {
	const isin = "US0846707026"
	client := &isinOnlyClient{isin: isin, symbol: "BRK-B"}
	err := teachJoinedShareClasses(context.Background(), client, map[string]string{"BRKB": isin})
	require.NoError(t, err)
	require.Equal(t, isin, client.gotISIN)
	require.Equal(t, "BRK-B", finance.YahooSymbol("BRKB"))

	// An empty ISIN on the hit does not teach a second ticker.
	client.symbol = "BF-B"
	client.isin = ""
	err = teachJoinedShareClasses(context.Background(), client, map[string]string{"BFB": "US1156372096"})
	require.NoError(t, err)
	require.Equal(t, "BFB", finance.YahooSymbol("BFB"))
}
