package main

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	grafana "github.com/grafana/grafana-plugin-sdk-go/backend"
	fbPb "github.com/lhjnilsson/foreverbull/pkg/pb"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

var exc1 = pb.GetExecutionResponse{
	Periods: []*pb.Period{
		{
			Date:    &fbPb.Date{},
			Returns: 1.0,
			Alpha:   func() *float64 { x := 2.0; return &x }(),
			Beta:    func() *float64 { x := 3.0; return &x }(),
			Sharpe:  func() *float64 { x := 4.0; return &x }(),
			Sortino: func() *float64 { x := 5.0; return &x }(),
		},
	},
}

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
		getExcMock.ReturnArguments = mock.Arguments{&exc1, nil}
	}

	test.uut = ds
}

func TestDatasource(t *testing.T) {
	suite.Run(t, new(DatasouceTest))
}

func (test *DatasouceTest) TestHandleExecutionMetric() {
	type TestCase struct {
		ExecutionID    string
		Metrics        []string
		ExpectedFields []string
	}

	testCases := []TestCase{
		{"123", []string{"returns"}, []string{"time", "returns"}},
		{"123", []string{"returns", "alpha"}, []string{"time", "returns", "alpha"}},
		{"123", []string{"returns", "alpha", "beta"}, []string{"time", "returns", "alpha", "beta"}},
		{"123", []string{"returns", "alpha", "beta", "sharpe"}, []string{"time", "returns", "alpha", "beta", "sharpe"}},
		{"123", []string{"returns", "alpha", "beta", "sharpe", "sortino"}, []string{"time", "returns", "alpha", "beta", "sharpe", "sortino"}},
	}
	for i, tc := range testCases {
		test.Run(fmt.Sprintf("Test_%d", i), func() {
			metrics, _ := json.Marshal(tc.Metrics)
			msg := json.RawMessage(fmt.Sprintf(`{"queryType": "executionMetric", "executionId": "%s", "metrics": %s}`, tc.ExecutionID, metrics))
			resp, err := test.uut.QueryData(context.Background(), &grafana.QueryDataRequest{
				Queries: []backend.DataQuery{
					{RefID: "A", QueryType: GetExecutionMetric, JSON: msg},
				},
			})
			test.Require().NoError(err)
			test.Require().NotNil(resp)
			test.Require().Equal(grafana.StatusOK, resp.Responses["A"].Status)

			test.Len(resp.Responses["A"].Frames, 1)
			test.Len(resp.Responses["A"].Frames[0].Fields, len(tc.ExpectedFields))
			test.Equal(tc.ExpectedFields, func() []string {
				var fields []string
				for _, f := range resp.Responses["A"].Frames[0].Fields {
					fields = append(fields, f.Name)
				}
				return fields
			}())
		})

	}
}
