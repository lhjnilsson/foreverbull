package strategy

import (
	"fmt"

	finance_pb "github.com/lhjnilsson/foreverbull/pkg/finance/pb"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/servicer"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/pb"
	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var Module = fx.Options( //nolint: gochecknoglobals
	fx.Provide(
		func() (finance_pb.MarketdataClient, error) {
			conn, err := grpc.NewClient("localhost:50055", grpc.WithTransportCredentials(
				insecure.NewCredentials(),
			))
			if err != nil {
				return nil, fmt.Errorf("failed to dial: %w", err)
			}

			// TO-DO: refactor so we properly can close the connection
			return finance_pb.NewMarketdataClient(conn), nil
		}),
	fx.Invoke(
		func(g *grpc.Server, md finance_pb.MarketdataClient) error {
			srv := servicer.NewStrategyServer(md)
			pb.RegisterStrategyServicerServer(g, srv)
			return nil
		}),
)
