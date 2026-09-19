package main

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	grafana "github.com/grafana/grafana-plugin-sdk-go/backend"
	fbPb "github.com/lhjnilsson/foreverbull/pkg/pb"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
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

	backend *pb.MockBacktestServicerClient
	uut     *Datasource
}

func (test *ResourceTest) SetupTest() {
	test.backend = pb.NewMockBacktestServicerClient(test.T())
	test.uut = newTestDatasource(test.T(), test.backend)
}

func (test *ResourceTest) TestExecutionLabel() {
	id := "84753648-1bba-4a76-8c5f-c0631204633e"
	label := executionLabel(&pb.Execution{
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
	test.backend.On("ListExecutions", mock.Anything, mock.Anything).Return(&pb.ListExecutionsResponse{
		Executions: []*pb.Execution{
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

	req := &grafana.CallResourceRequest{
		Method: http.MethodGet,
		Path:   "executions",
	}

	var r mockCallResourceResponseSender
	err := test.uut.CallResource(context.Background(), req, &r)
	test.Require().NoError(err, "fail to call resource")

	test.Equal(http.StatusOK, r.response.Status, "unexpected status")

	var got []ResourceDefinition
	test.Require().NoError(json.Unmarshal(r.response.Body, &got))
	test.Equal([]ResourceDefinition{
		{Value: id, Label: "spy 2023-01-03–2023-12-25 84753648"},
	}, got)
}

func (test *ResourceTest) TestListMetrics() {
	req := &grafana.CallResourceRequest{
		Method: http.MethodGet,
		Path:   "metrics",
	}

	var r mockCallResourceResponseSender
	err := test.uut.CallResource(context.Background(), req, &r)
	test.Require().NoError(err, "fail to call resource")

	test.Equal(http.StatusOK, r.response.Status, "unexpected status")

	var got []ResourceDefinition
	test.Require().NoError(json.Unmarshal(r.response.Body, &got))
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
}
