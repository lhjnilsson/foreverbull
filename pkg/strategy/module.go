package strategy

import (
	finance_pb "github.com/lhjnilsson/foreverbull/pkg/finance/pb"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/servicer"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/pb"
	"go.uber.org/fx"
	"google.golang.org/grpc"
)

var Module = fx.Options( //nolint: gochecknoglobals
	fx.Provide(
		func() (finance_pb.MarketdataClient, error) {

			return finance_pb.NewMarketdataClient(nil), nil
		}),
	fx.Invoke(
		func(g *grpc.Server, md finance_pb.MarketdataClient) error {
			srv := servicer.NewStrategyServer(md)
			pb.RegisterStrategyServicerServer(g, srv)
			return nil
		}),
)
