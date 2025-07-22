package api

import (
	"fmt"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewMarketdataClient() (service.MarketdataClient, error) {
	url := fmt.Sprintf("localhost:%s", environment.GetGRPCPort())
	conn, err := grpc.NewClient(url,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return service.NewMarketdataClient(conn), nil
}
