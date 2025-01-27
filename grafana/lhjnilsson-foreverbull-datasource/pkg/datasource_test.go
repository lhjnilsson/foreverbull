package main

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	grafana "github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/pb"
	fbPb "github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

var exc1 = pb.GetExecutionResponse{
	Periods: []*pb.Period{
		{
			Date:    fbPb.Date{},
			Returns: 1.0,
		},
	},
}
var exc2 = pb.GetBacktestResponse{}

type DatasouceTest struct {
	suite.Suite

	listener *bufconn.Listener
	server   *grpc.Server
	backend  *pb.MockBacktestServicerClient
	uut      *Datasource
}

func (test *DatasouceTest) SetupSuite() {
	settings := grafana.DataSourceInstanceSettings{}

	dsInstance, err := NewDatasource(context.Background(), settings)
	test.Require().NoError(err, "fail to create datasource")

	ds, isDataSource := dsInstance.(*Datasource)
	test.Require().True(isDataSource, "Datasource must be an instance of Datasource")

	test.listener = bufconn.Listen(1024 * 1024)
	test.server = grpc.NewServer()
	test.Require().NoError(err)

	test.backend = pb.NewMockBacktestServicerClient(test.T())
	ds.backend = test.backend

	getExcMock := test.backend.On("GetExecution", mock.Anything, mock.Anything)
	getExcMock.RunFn = func(args mock.Arguments) {
		fmt.Println("ARGS: ", args.Get(1))
		getExcMock.ReturnArguments = mock.Arguments{&exc1, nil}
	}

	test.uut = ds
}

func TestDatasource(t *testing.T) {
	suite.Run(t, new(DatasouceTest))
}

func (test *DatasouceTest) TestHandleExecutionMetric() {

	msg := json.RawMessage(`{"queryType": "executionMetric", "executionId": "123", "metrics": ["returns"]}`)

	resp, err := test.uut.QueryData(context.Background(), &grafana.QueryDataRequest{
		Queries: []backend.DataQuery{
			{RefID: "A", QueryType: GetExecutionMetric, JSON: msg},
		},
	})

	test.Require().NoError(err)

	fmt.Println("RESP: Resp", resp)
}
