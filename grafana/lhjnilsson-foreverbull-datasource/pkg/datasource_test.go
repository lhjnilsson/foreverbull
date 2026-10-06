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
	backtestpb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	backtestsvc "github.com/lhjnilsson/foreverbull/pkg/pb/backtest/service"
	strategysvc "github.com/lhjnilsson/foreverbull/pkg/pb/strategy/service"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

func floatPtr(v float64) *float64 {
	return &v
}

var exc1 = backtestsvc.GetExecutionResponse{
	Execution: &backtestpb.Execution{
		Id:       "123",
		Backtest: "spy",
		StartDate: &fbPb.Date{
			Year:  2023,
			Month: 1,
			Day:   3,
		},
		EndDate: &fbPb.Date{
			Year:  2023,
			Month: 12,
			Day:   25,
		},
	},
	Periods: []*backtestpb.Period{
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
			CapitalUsed:           25,
			GrossLeverage:         1.25,
			NetLeverage:           0.75,
			MaxLeverage:           1.5,
			LongExposure:          80,
			ShortExposure:         -20,
			EndingExposure:        60,
			StartingExposure:      55,
		},
	},
}

func newTestDatasource(t *testing.T, backendClient backtestAPI, strategyClient strategyAPI) *Datasource {
	t.Helper()
	mux := datasource.NewQueryTypeMux()
	ds := &Datasource{
		queryMux: mux,
		backend:  backendClient,
		strategy: strategyClient,
		log:      log.DefaultLogger.With("datasource", "foreverbull"),
	}
	ds.registerResources()
	mux.HandleFunc(GetExecutionMetric, ds.HandleGetExecutionMetric)
	return ds
}

type DatasouceTest struct {
	suite.Suite

	backend  *backtestsvc.MockBacktestServicerClient
	strategy *strategysvc.MockStrategyServicerClient
	uut      *Datasource
}

func (test *DatasouceTest) SetupTest() {
	test.backend = backtestsvc.NewMockBacktestServicerClient(test.T())
	test.strategy = strategysvc.NewMockStrategyServicerClient(test.T())
	test.uut = newTestDatasource(test.T(), test.backend, test.strategy)
}

func TestDatasource(t *testing.T) {
	suite.Run(t, new(DatasouceTest))
}

func (test *DatasouceTest) TestCheckHealthOK() {
	test.backend.On("ListBacktests", mock.Anything, mock.Anything).
		Return(&backtestsvc.ListBacktestsResponse{}, nil)

	res, err := test.uut.CheckHealth(context.Background(), &grafana.CheckHealthRequest{})
	test.Require().NoError(err)
	test.Equal(grafana.HealthStatusOk, res.Status)
	test.Equal("Data source is working", res.Message)
}

func (test *DatasouceTest) TestCheckHealthBrokerError() {
	test.backend.On("ListBacktests", mock.Anything, mock.Anything).
		Return((*backtestsvc.ListBacktestsResponse)(nil), errors.New("connection refused"))

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
		{
			JSON: `{"queryType":"GetExecutionMetric","execution":{"id":"123"},"metrics":[` +
				`{"name":"gross_leverage"},{"name":"net_leverage"},{"name":"max_leverage"},` +
				`{"name":"long_exposure"},{"name":"short_exposure"},{"name":"ending_exposure"},{"name":"starting_exposure"}]}`,
			ExpectedFields: []string{
				"time", "gross_leverage", "net_leverage", "max_leverage",
				"long_exposure", "short_exposure", "ending_exposure", "starting_exposure",
			},
		},
	}
	for i, tc := range testCases {
		test.Run(fmt.Sprintf("Test_%d", i), func() {
			test.backend.ExpectedCalls = nil
			test.backend.On("GetExecution", mock.Anything, mock.MatchedBy(func(req *backtestsvc.GetExecutionRequest) bool {
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

func (test *DatasouceTest) TestMetricUnitsMatchStoredNumbers() {
	test.backend.On("GetExecution", mock.Anything, mock.Anything).Return(&exc1, nil).Once()

	resp, err := test.uut.QueryData(context.Background(), &grafana.QueryDataRequest{
		Queries: []backend.DataQuery{
			{
				RefID:     "A",
				QueryType: GetExecutionMetric,
				JSON: json.RawMessage(`{"queryType":"GetExecutionMetric","execution":{"id":"123"},"metrics":[` +
					`{"name":"returns"},{"name":"portfolio_value"},{"name":"max_drawdown"},` +
					`{"name":"sharpe"},{"name":"benchmark"},{"name":"long_value"},` +
					`{"name":"pnl"},{"name":"cash"},{"name":"alpha"},{"name":"beta"},{"name":"sortino"},{"name":"capital_used"},` +
					`{"name":"gross_leverage"},{"name":"net_leverage"},{"name":"max_leverage"},` +
					`{"name":"long_exposure"},{"name":"short_exposure"},{"name":"ending_exposure"},{"name":"starting_exposure"}]}`),
			},
		},
	})
	test.Require().NoError(err)
	fields := resp.Responses["A"].Frames[0].Fields
	units := map[string]string{}
	values := map[string]interface{}{}
	for _, field := range fields {
		if field.Config != nil {
			units[field.Name] = field.Config.Unit
		}
		if field.Len() > 0 {
			values[field.Name] = field.At(0)
		}
	}
	// Fractions stay fractions; percentunit is display-only. Ratios stay ratios. Currency stays currency.
	test.Equal("percentunit", units["returns"])
	test.Equal(1.0, values["returns"])
	test.Equal("percentunit", units["max_drawdown"])
	test.Equal(-0.2, values["max_drawdown"])
	test.Equal("percentunit", units["benchmark"])
	test.Equal(0.5, *values["benchmark"].(*float64))
	test.Equal("none", units["sharpe"])
	test.Equal(4.0, *values["sharpe"].(*float64))
	test.Equal("currencyUSD", units["portfolio_value"])
	test.Equal(100.0, values["portfolio_value"])
	test.Equal("currencyUSD", units["long_value"])
	test.Equal(0.0, values["long_value"])
	test.Equal("currencyUSD", units["pnl"])
	test.Equal(10.0, values["pnl"])
	test.Equal("currencyUSD", units["cash"])
	test.Equal(1000.0, values["cash"])
	test.Equal("currencyUSD", units["capital_used"])
	test.Equal(25.0, values["capital_used"])
	test.Equal("percentunit", units["alpha"])
	test.Equal(2.0, *values["alpha"].(*float64))
	test.Equal("none", units["beta"])
	test.Equal(3.0, *values["beta"].(*float64))
	test.Equal("none", units["sortino"])
	test.Equal(5.0, *values["sortino"].(*float64))
	test.Equal("none", units["gross_leverage"])
	test.Equal(1.25, values["gross_leverage"])
	test.Equal("none", units["net_leverage"])
	test.Equal(0.75, values["net_leverage"])
	test.Equal("none", units["max_leverage"])
	test.Equal(1.5, values["max_leverage"])
	test.Equal("currencyUSD", units["long_exposure"])
	test.Equal(80.0, values["long_exposure"])
	test.Equal("currencyUSD", units["short_exposure"])
	test.Equal(-20.0, values["short_exposure"])
	test.Equal("currencyUSD", units["ending_exposure"])
	test.Equal(60.0, values["ending_exposure"])
	test.Equal("currencyUSD", units["starting_exposure"])
	test.Equal(55.0, values["starting_exposure"])
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

func (test *DatasouceTest) TestHandleExecutionMetricOverlay() {
	execA := exc1
	execA.Execution = &backtestpb.Execution{
		Id:       "exec-a",
		Backtest: "spy",
		StartDate: &fbPb.Date{
			Year:  2023,
			Month: 1,
			Day:   3,
		},
		EndDate: &fbPb.Date{
			Year:  2023,
			Month: 12,
			Day:   25,
		},
	}
	execB := exc1
	execB.Execution = &backtestpb.Execution{
		Id:       "exec-b",
		Backtest: "eu",
		StartDate: &fbPb.Date{
			Year:  2023,
			Month: 1,
			Day:   3,
		},
		EndDate: &fbPb.Date{
			Year:  2023,
			Month: 12,
			Day:   25,
		},
	}

	test.backend.On("GetExecution", mock.Anything, mock.MatchedBy(func(req *backtestsvc.GetExecutionRequest) bool {
		return req.GetExecutionId() == "exec-a"
	})).Return(&execA, nil).Once()
	test.backend.On("GetExecution", mock.Anything, mock.MatchedBy(func(req *backtestsvc.GetExecutionRequest) bool {
		return req.GetExecutionId() == "exec-b"
	})).Return(&execB, nil).Once()

	resp, err := test.uut.QueryData(context.Background(), &grafana.QueryDataRequest{
		Queries: []backend.DataQuery{
			{
				RefID:     "A",
				QueryType: GetExecutionMetric,
				JSON:      json.RawMessage(`{"queryType":"GetExecutionMetric","execution":{"id":"exec-a"},"metrics":[{"name":"returns"}]}`),
			},
			{
				RefID:     "B",
				QueryType: GetExecutionMetric,
				JSON:      json.RawMessage(`{"queryType":"GetExecutionMetric","execution":{"id":"exec-b"},"metrics":[{"name":"returns"}]}`),
			},
		},
	})
	test.Require().NoError(err)
	test.Equal(grafana.StatusOK, resp.Responses["A"].Status)
	test.Equal(grafana.StatusOK, resp.Responses["B"].Status)

	test.Equal("spy 2023-01-03–2023-12-25 exec-a", resp.Responses["A"].Frames[0].Name)
	test.Equal("eu 2023-01-03–2023-12-25 exec-b", resp.Responses["B"].Frames[0].Name)
	test.Equal("spy 2023-01-03–2023-12-25 exec-a", resp.Responses["A"].Frames[0].Fields[1].Labels["execution"])
	test.Equal("eu 2023-01-03–2023-12-25 exec-b", resp.Responses["B"].Frames[0].Fields[1].Labels["execution"])
}
