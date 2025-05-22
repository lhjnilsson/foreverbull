package strategy

import (
	"fmt"

	financeServicePb "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy/service"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/api"
	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var Module = fx.Options( //nolint: gochecknoglobals
	fx.Provide(
		func() (financeServicePb.MarketdataClient, error) {
			conn, err := grpc.NewClient("localhost:50055", grpc.WithTransportCredentials(
				insecure.NewCredentials(),
			))
			if err != nil {
				return nil, fmt.Errorf("failed to dial: %w", err)
			}

			// TO-DO: refactor so we properly can close the connection
			return financeServicePb.NewMarketdataClient(conn), nil
		}),
	fx.Invoke(
		func(g *grpc.Server, md financeServicePb.MarketdataClient) error {
			srv := api.NewStrategyServer(md)
			pb.RegisterStrategyServicerServer(g, srv)
			return nil
		}),
)
