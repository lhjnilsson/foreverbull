package main

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	grafana "github.com/grafana/grafana-plugin-sdk-go/backend"
	fbPb "github.com/lhjnilsson/foreverbull/pkg/pb"
	backtestpb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	backtestsvc "github.com/lhjnilsson/foreverbull/pkg/pb/backtest/service"
	strategypb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy"
	strategysvc "github.com/lhjnilsson/foreverbull/pkg/pb/strategy/service"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockCallResourceResponseSender struct {
	response *grafana.CallResourceResponse
}

func (s *mockCallResourceResponseSender) Send(response *grafana.CallResourceResponse) error {
	s.response = response
	return nil
}

func TestResources(t *testing.T) {
	suite.Run(t, new(ResourceTest))
}

type ResourceTest struct {
	suite.Suite

	backend  *backtestsvc.MockBacktestServicerClient
	strategy *strategysvc.MockStrategyServicerClient
	uut      *Datasource
}

func (test *ResourceTest) SetupTest() {
	test.backend = backtestsvc.NewMockBacktestServicerClient(test.T())
	test.strategy = strategysvc.NewMockStrategyServicerClient(test.T())
	test.uut = newTestDatasource(test.T(), test.backend, test.strategy)
}

func strPtr(s string) *string {
	return &s
}

func (test *ResourceTest) callResource(path, url string, body []byte) []ResourceDefinition {
	test.T().Helper()
	req := &grafana.CallResourceRequest{
		Method: http.MethodGet,
		Path:   path,
		URL:    url,
		Body:   body,
	}
	var r mockCallResourceResponseSender
	err := test.uut.CallResource(context.Background(), req, &r)
	test.Require().NoError(err, "fail to call resource")
	test.Equal(http.StatusOK, r.response.Status, "unexpected status: %s", string(r.response.Body))

	var got []ResourceDefinition
	test.Require().NoError(json.Unmarshal(r.response.Body, &got))
	return got
}

func (test *ResourceTest) TestExecutionLabel() {
	id := "84753648-1bba-4a76-8c5f-c0631204633e"
	label := executionLabel(&backtestpb.Execution{
		Id:       id,
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
	})
	test.Equal("spy 2023-01-03–2023-12-25 84753648", label)
	test.Equal("84753648", shortExecutionID(id))
}

func (test *ResourceTest) TestListExecutions() {
	id := "84753648-1bba-4a76-8c5f-c0631204633e"
	test.backend.On("ListExecutions", mock.Anything, mock.Anything).Return(&backtestsvc.ListExecutionsResponse{
		Executions: []*backtestpb.Execution{
			{
				Id:       id,
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
		},
	}, nil)

	got := test.callResource("executions", "/executions", nil)
	test.Equal([]ResourceDefinition{
		{
			Value: id,
			Label: "spy 2023-01-03–2023-12-25 84753648",
			Start: "2023-01-03",
			End:   "2023-12-25",
		},
	}, got)
}

func (test *ResourceTest) TestListExecutionsFiltersByBacktest() {
	spyID := "84753648-1bba-4a76-8c5f-c0631204633e"
	test.backend.On("ListExecutions", mock.Anything, mock.MatchedBy(func(req *backtestsvc.ListExecutionsRequest) bool {
		return req.GetBacktest() == "spy" && req.GetSessionId() == ""
	})).Return(&backtestsvc.ListExecutionsResponse{
		Executions: []*backtestpb.Execution{
			{
				Id:       spyID,
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
		},
	}, nil)

	got := test.callResource("executions", "/executions", []byte(`{"backtest":"spy"}`))
	test.Len(got, 1)
	test.Equal(spyID, got[0].Value)
	test.Equal("spy 2023-01-03–2023-12-25 84753648", got[0].Label)
	test.NotContains(got[0].Label, "eu")
}

func (test *ResourceTest) TestListExecutionsFiltersByStrategyExperiments() {
	execID := "exec-spy"
	otherID := "exec-other"
	test.strategy.On("ListExperiments", mock.Anything, mock.MatchedBy(func(req *strategysvc.ListExperimentsRequest) bool {
		return req.GetStrategyName() == "mean-rev" && req.GetVersionId() == "ver-1"
	})).Return(&strategysvc.ListExperimentsResponse{
		Experiments: []*strategypb.Experiment{
			{
				Id:           "exp-1",
				StrategyName: "mean-rev",
				VersionId:    "ver-1",
				VersionRev:   2,
				BacktestName: "spy",
				ExecutionId:  strPtr(execID),
			},
		},
	}, nil)
	test.backend.On("ListExecutions", mock.Anything, mock.Anything).Return(&backtestsvc.ListExecutionsResponse{
		Executions: []*backtestpb.Execution{
			{Id: execID, Backtest: "spy"},
			{Id: otherID, Backtest: "eu"},
		},
	}, nil)

	got := test.callResource("executions", "/executions?strategy=mean-rev&version=ver-1", nil)
	test.Equal([]ResourceDefinition{
		{Value: execID, Label: "spy exec-spy"},
	}, got)
}

func (test *ResourceTest) TestListMetrics() {
	got := test.callResource("metrics", "/metrics", nil)
	values := make([]string, 0, len(got))
	for _, def := range got {
		values = append(values, def.Value)
	}
	test.Contains(values, string(Returns))
	test.Contains(values, string(PnL))
	test.Contains(values, string(Cash))
	test.Contains(values, string(MaxDrawdown))
	test.Contains(values, string(Benchmark))
	test.Contains(values, string(PortfolioValue))
	test.Contains(values, string(Alpha))
	test.Contains(values, string(Beta))
	test.Contains(values, string(Sortino))
	test.Contains(values, string(CapitalUsed))
	test.Contains(values, string(GrossLeverage))
	test.Contains(values, string(NetLeverage))
	test.Contains(values, string(MaxLeverage))
	test.Contains(values, string(LongExposure))
	test.Contains(values, string(ShortExposure))
	test.Contains(values, string(EndingExposure))
	test.Contains(values, string(StartingExposure))
}

func (test *ResourceTest) TestListStrategies() {
	test.strategy.On("ListStrategies", mock.Anything, mock.Anything).Return(&strategysvc.ListStrategiesResponse{
		Strategies: []*strategypb.Strategy{
			{Id: "s1", Name: "mean-rev"},
			{Id: "s2", Name: "momentum"},
		},
	}, nil)

	got := test.callResource("strategies", "/strategies", nil)
	test.Equal([]ResourceDefinition{
		{Value: "mean-rev", Label: "mean-rev", Description: "s1"},
		{Value: "momentum", Label: "momentum", Description: "s2"},
	}, got)
}

func (test *ResourceTest) TestListExecutionsStrategyFilterUnimplemented() {
	test.strategy.On("ListExperiments", mock.Anything, mock.Anything).
		Return((*strategysvc.ListExperimentsResponse)(nil), status.Error(codes.Unimplemented, "method ListExperiments not implemented"))

	got := test.callResource("executions", "/executions?strategy=mean-rev", nil)
	test.Empty(got)
}

func (test *ResourceTest) TestListStrategiesUnimplemented() {
	test.strategy.On("ListStrategies", mock.Anything, mock.Anything).
		Return((*strategysvc.ListStrategiesResponse)(nil), status.Error(codes.Unimplemented, "method ListStrategies not implemented"))

	got := test.callResource("strategies", "/strategies", nil)
	test.Empty(got)
}

func (test *ResourceTest) TestListStrategiesMissingClient() {
	test.uut.strategy = nil
	got := test.callResource("strategies", "/strategies", nil)
	test.Empty(got)
}

func (test *ResourceTest) TestListExperiments() {
	execID := "84753648-1bba-4a76-8c5f-c0631204633e"
	test.strategy.On("ListExperiments", mock.Anything, mock.MatchedBy(func(req *strategysvc.ListExperimentsRequest) bool {
		return req.GetStrategyName() == "mean-rev" && req.GetVersionId() == "ver-1"
	})).Return(&strategysvc.ListExperimentsResponse{
		Experiments: []*strategypb.Experiment{
			{
				Id:           "exp-spy",
				StrategyName: "mean-rev",
				VersionId:    "ver-1",
				VersionRev:   2,
				BacktestName: "spy",
				ExecutionId:  strPtr(execID),
			},
			{
				Id:           "exp-running",
				StrategyName: "mean-rev",
				VersionId:    "ver-1",
				VersionRev:   2,
				BacktestName: "eu",
			},
		},
	}, nil)

	got := test.callResource("experiments", "/experiments?strategy=mean-rev&version=ver-1", nil)
	test.Equal([]ResourceDefinition{
		{Value: execID, Label: "mean-rev / rev 2 / spy", Description: "exp-spy"},
	}, got)
}

func (test *ResourceTest) TestListExperimentsUnimplemented() {
	test.strategy.On("ListExperiments", mock.Anything, mock.Anything).
		Return((*strategysvc.ListExperimentsResponse)(nil), status.Error(codes.Unimplemented, "method ListExperiments not implemented"))

	got := test.callResource("experiments", "/experiments", nil)
	test.Empty(got)
}

func (test *ResourceTest) TestListVersions() {
	test.strategy.On("ListExperiments", mock.Anything, mock.MatchedBy(func(req *strategysvc.ListExperimentsRequest) bool {
		return req.GetStrategyName() == "mean-rev"
	})).Return(&strategysvc.ListExperimentsResponse{
		Experiments: []*strategypb.Experiment{
			{VersionId: "ver-1", VersionRev: 1, StrategyName: "mean-rev", BacktestName: "spy"},
			{VersionId: "ver-1", VersionRev: 1, StrategyName: "mean-rev", BacktestName: "eu"},
			{VersionId: "ver-2", VersionRev: 2, StrategyName: "mean-rev", BacktestName: "spy"},
		},
	}, nil)

	got := test.callResource("versions", "/versions?strategy=mean-rev", nil)
	test.Equal([]ResourceDefinition{
		{Value: "ver-1", Label: "rev 1", Description: "mean-rev"},
		{Value: "ver-2", Label: "rev 2", Description: "mean-rev"},
	}, got)
}

func (test *ResourceTest) TestListExperimentsFromJSONBody() {
	execID := "exec-eu"
	test.strategy.On("ListExperiments", mock.Anything, mock.MatchedBy(func(req *strategysvc.ListExperimentsRequest) bool {
		return req.GetStrategyName() == "mean-rev" && req.GetBacktestName() == "eu"
	})).Return(&strategysvc.ListExperimentsResponse{
		Experiments: []*strategypb.Experiment{
			{
				Id:           "exp-eu",
				StrategyName: "mean-rev",
				VersionId:    "ver-2",
				VersionRev:   2,
				BacktestName: "eu",
				ExecutionId:  strPtr(execID),
			},
		},
	}, nil)

	got := test.callResource("experiments", "/experiments", []byte(`{"strategy":"mean-rev","backtest_name":"eu"}`))
	test.Equal([]ResourceDefinition{
		{Value: execID, Label: "mean-rev / rev 2 / eu", Description: "exp-eu"},
	}, got)
}

func (test *ResourceTest) callRaw(method, path, rawURL string, body []byte) *grafana.CallResourceResponse {
	test.T().Helper()
	req := &grafana.CallResourceRequest{
		Method: method,
		Path:   path,
		URL:    rawURL,
		Body:   body,
	}
	var sender mockCallResourceResponseSender
	err := test.uut.CallResource(context.Background(), req, &sender)
	test.Require().NoError(err)
	test.Require().NotNil(sender.response)
	return sender.response
}

func (test *ResourceTest) TestListBacktests() {
	test.backend.On("ListBacktests", mock.Anything, mock.Anything).Return(&backtestsvc.ListBacktestsResponse{
		Backtests: []*backtestpb.Backtest{
			{Name: "spy", Benchmark: "SPY"},
			{Name: ""},
			{Name: "eu-small", Benchmark: "EXUS"},
		},
	}, nil)

	rsp := test.callRaw(http.MethodGet, "backtests", "/backtests", nil)
	test.Equal(http.StatusOK, rsp.Status)
	test.NotContains(string(rsp.Body), "SPY")
	test.NotContains(string(rsp.Body), "EXUS")
	var got []ResourceDefinition
	test.Require().NoError(json.Unmarshal(rsp.Body, &got))
	test.Equal([]ResourceDefinition{
		{Value: "spy", Label: "spy"},
		{Value: "eu-small", Label: "eu-small"},
	}, got)
}

func (test *ResourceTest) TestGetExecutionArguments() {
	id := "84753648-1bba-4a76-8c5f-c0631204633e"
	test.backend.On("GetExecution", mock.Anything, mock.MatchedBy(func(req *backtestsvc.GetExecutionRequest) bool {
		return req.GetExecutionId() == id
	})).Return(&backtestsvc.GetExecutionResponse{
		Execution: &backtestpb.Execution{
			Id:       id,
			Backtest: "spy",
			Session:  "sess-1",
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
		Functions: []*backtestpb.ExecutionFunction{
			{
				Name: "rebalance",
				Parameters: []*backtestpb.ExecutionFunction_Parameter{
					{Key: "fastperiod", Value: "12", ValueType: "int"},
					{Key: "oversold", Value: "", ValueType: "int"},
				},
			},
			{Name: "screen", Parameters: []*backtestpb.ExecutionFunction_Parameter{}},
		},
	}, nil)

	test.backend.On("GetBacktest", mock.Anything, mock.MatchedBy(func(req *backtestsvc.GetBacktestRequest) bool {
		return req.GetName() == "spy"
	})).Return(&backtestsvc.GetBacktestResponse{
		Name:     "spy",
		Backtest: &backtestpb.Backtest{Name: "spy", Benchmark: "SPY"},
	}, nil)

	rsp := test.callRaw(http.MethodGet, "execution", "/execution?id="+id, nil)
	test.Equal(http.StatusOK, rsp.Status)
	body := string(rsp.Body)
	test.NotContains(body, "database")
	test.NotContains(body, "brokerPort")
	test.NotContains(body, "namespacePort")
	test.NotContains(body, "databaseURL")
	var got executionArgumentsView
	test.Require().NoError(json.Unmarshal(rsp.Body, &got))
	test.Equal(executionArgumentsView{
		ID:        id,
		Session:   "sess-1",
		Backtest:  "spy",
		Benchmark: "SPY",
		Start:     "2023-01-03",
		End:       "2023-12-25",
		Functions: []executionFunctionView{
			{
				Name: "rebalance",
				Parameters: []executionParameterView{
					{Key: "fastperiod", Value: "12", ValueType: "int"},
					{Key: "oversold", Value: "", ValueType: "int"},
				},
			},
			{Name: "screen", Parameters: []executionParameterView{}},
		},
	}, got)
}

func (test *ResourceTest) TestGetExecutionArgumentsLegacyEmpty() {
	id := "legacy-exec"
	test.backend.On("GetExecution", mock.Anything, mock.Anything).Return(&backtestsvc.GetExecutionResponse{
		Execution: &backtestpb.Execution{
			Id:       id,
			Backtest: "spy",
			Session:  "sess-old",
			StartDate: &fbPb.Date{
				Year:  2020,
				Month: 1,
				Day:   1,
			},
			EndDate: &fbPb.Date{
				Year:  2020,
				Month: 12,
				Day:   31,
			},
		},
	}, nil)

	test.backend.On("GetBacktest", mock.Anything, mock.Anything).
		Return((*backtestsvc.GetBacktestResponse)(nil), status.Error(codes.NotFound, "missing"))

	rsp := test.callRaw(http.MethodPost, "execution", "/execution", []byte(`{"execution":"legacy-exec"}`))
	test.Equal(http.StatusOK, rsp.Status)
	var got executionArgumentsView
	test.Require().NoError(json.Unmarshal(rsp.Body, &got))
	test.Equal("legacy-exec", got.ID)
	test.Equal("sess-old", got.Session)
	test.Equal("spy", got.Backtest)
	test.Equal("", got.Benchmark)
	test.Equal("2020-01-01", got.Start)
	test.Equal("2020-12-31", got.End)
	test.NotNil(got.Functions)
	test.Empty(got.Functions)
	test.Contains(string(rsp.Body), `"functions":[]`)
	test.NotContains(string(rsp.Body), "fastperiod")
	test.NotContains(string(rsp.Body), "database")
}

func (test *ResourceTest) TestGetExecutionArgumentsRequiresID() {
	rsp := test.callRaw(http.MethodGet, "execution", "/execution", nil)
	test.Equal(http.StatusBadRequest, rsp.Status)
}
