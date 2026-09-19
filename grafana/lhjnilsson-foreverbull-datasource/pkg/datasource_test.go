package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	grafana "github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	fbPb "github.com/lhjnilsson/foreverbull/pkg/pb"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

func floatPtr(v float64) *float64 {
	return &v
}

var exc1 = pb.GetExecutionResponse{
	Periods: []*pb.Period{
		{
			Date:                  &fbPb.Date{Year: 2024, Month: 1, Day: 2},
			Returns:               1.0,
			PNL:                   10.0,
			PortfolioValue:        100.0,
			EndingCash:            1000.0,
			MaxDrawdown:           -0.2,
			BenchmarkPeriodReturn: floatPtr(0.5),
			Alpha:                 floatPtr(2.0),
			Beta:                  floatPtr(3.0),
			Sharpe:                floatPtr(4.0),
			Sortino:               floatPtr(5.0),
		},
	},
}

func newTestDatasource(t *testing.T, backendClient pb.BacktestServicerClient) *Datasource {
	t.Helper()
	mux := datasource.NewQueryTypeMux()
	ds := &Datasource{
		queryMux: mux,
		backend:  backendClient,
		log:      log.DefaultLogger.With("datasource", "foreverbull"),
	}
	ds.registerResources()
	mux.HandleFunc(GetExecutionMetric, ds.HandleGetExecutionMetric)
	return ds
}

type DatasouceTest struct {
	suite.Suite

	backend *pb.MockBacktestServicerClient
	uut     *Datasource
}

func (test *DatasouceTest) SetupTest() {
	test.backend = pb.NewMockBacktestServicerClient(test.T())
	test.uut = newTestDatasource(test.T(), test.backend)
}

func TestDatasource(t *testing.T) {
	suite.Run(t, new(DatasouceTest))
}

func (test *DatasouceTest) TestCheckHealthOK() {
	test.backend.On("ListBacktests", mock.Anything, mock.Anything).
		Return(&pb.ListBacktestsResponse{}, nil)

	res, err := test.uut.CheckHealth(context.Background(), &grafana.CheckHealthRequest{})
	test.Require().NoError(err)
	test.Equal(grafana.HealthStatusOk, res.Status)
	test.Equal("Data source is working", res.Message)
}

func (test *DatasouceTest) TestCheckHealthBrokerError() {
	test.backend.On("ListBacktests", mock.Anything, mock.Anything).
		Return((*pb.ListBacktestsResponse)(nil), errors.New("connection refused"))

	res, err := test.uut.CheckHealth(context.Background(), &grafana.CheckHealthRequest{})
	test.Require().NoError(err)
	test.Equal(grafana.HealthStatusError, res.Status)
	test.Contains(res.Message, "broker unreachable")
}

func (test *DatasouceTest) TestCheckHealthMissingClient() {
	test.uut.backend = nil
	res, err := test.uut.CheckHealth(context.Background(), &grafana.CheckHealthRequest{})
	test.Require().NoError(err)
	test.Equal(grafana.HealthStatusError, res.Status)
}

func (test *DatasouceTest) TestDisposeClosesOwnedConn() {
	dsInstance, err := NewDatasource(context.Background(), grafana.DataSourceInstanceSettings{})
	test.Require().NoError(err)
	ds, ok := dsInstance.(*Datasource)
	test.Require().True(ok)
	test.Require().NotNil(ds.conn)

	ds.Dispose()
	test.Nil(ds.conn)
	ds.Dispose()
}

func (test *DatasouceTest) TestParseQueryModelAcceptsExecutionAndExecutionID() {
	cases := []struct {
		name string
		raw  string
		id   string
	}{
		{
			name: "canonical execution object",
			raw:  `{"queryType":"GetExecutionMetric","execution":{"id":"abc"},"metrics":[{"name":"returns"}]}`,
			id:   "abc",
		},
		{
			name: "typescript capital ID",
			raw:  `{"queryType":"GetExecutionMetric","execution":{"ID":"abc"},"metrics":[{"name":"returns"}]}`,
			id:   "abc",
		},
		{
			name: "legacy executionId string",
			raw:  `{"queryType":"GetExecutionMetric","executionId":"abc","metrics":[{"name":"returns"}]}`,
			id:   "abc",
		},
		{
			name: "execution object wins over executionId",
			raw:  `{"execution":{"id":"canonical"},"executionId":"legacy","metrics":[{"name":"returns"}]}`,
			id:   "canonical",
		},
	}
	for _, tc := range cases {
		test.Run(tc.name, func() {
			var qm queryModel
			test.Require().NoError(json.Unmarshal([]byte(tc.raw), &qm))
			test.Equal(tc.id, qm.ExecutionID)
			test.Equal(QueryMetric("returns"), qm.Metrics[0].Name)
		})
	}
}

func (test *DatasouceTest) TestHandleExecutionMetric() {
	type TestCase struct {
		JSON           string
		ExpectedFields []string
	}

	testCases := []TestCase{
		{
			JSON:           `{"queryType":"GetExecutionMetric","execution":{"id":"123"},"metrics":[{"name":"returns"}]}`,
			ExpectedFields: []string{"time", "returns"},
		},
		{
			JSON:           `{"queryType":"GetExecutionMetric","execution":{"id":"123"},"metrics":[{"name":"returns"},{"name":"alpha"}]}`,
			ExpectedFields: []string{"time", "returns", "alpha"},
		},
		{
			JSON:           `{"queryType":"GetExecutionMetric","execution":{"id":"123"},"metrics":[{"name":"pnl"},{"name":"cash"},{"name":"max_drawdown"},{"name":"benchmark"}]}`,
			ExpectedFields: []string{"time", "pnl", "cash", "max_drawdown", "benchmark"},
		},
		{
			JSON:           `{"queryType":"GetExecutionMetric","executionId":"123","metrics":[{"name":"portfolio_value"}]}`,
			ExpectedFields: []string{"time", "portfolio_value"},
		},
	}
	for i, tc := range testCases {
		test.Run(fmt.Sprintf("Test_%d", i), func() {
			test.backend.ExpectedCalls = nil
			test.backend.On("GetExecution", mock.Anything, mock.MatchedBy(func(req *pb.GetExecutionRequest) bool {
				return req.GetExecutionId() == "123"
			})).Return(&exc1, nil).Once()

			resp, err := test.uut.QueryData(context.Background(), &grafana.QueryDataRequest{
				Queries: []backend.DataQuery{
					{RefID: "A", QueryType: GetExecutionMetric, JSON: json.RawMessage(tc.JSON)},
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

func (test *DatasouceTest) TestHandleExecutionMetricMissingExecution() {
	resp, err := test.uut.QueryData(context.Background(), &grafana.QueryDataRequest{
		Queries: []backend.DataQuery{
			{RefID: "A", QueryType: GetExecutionMetric, JSON: json.RawMessage(`{"queryType":"GetExecutionMetric","metrics":[{"name":"returns"}]}`)},
		},
	})
	test.Require().NoError(err)
	test.Equal(grafana.StatusBadRequest, resp.Responses["A"].Status)
	test.Contains(resp.Responses["A"].Error.Error(), "executionId is missing")
}
