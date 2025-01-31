package main

import (
	"context"
	"net/http"
	"testing"

	grafana "github.com/grafana/grafana-plugin-sdk-go/backend"
	backtestPb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
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

	listener *bufconn.Listener
	server   *grpc.Server
	backend  *pb.MockBacktestServicerClient
	uut      *Datasource
}

func (test *ResourceTest) SetupTest() {
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

	test.uut = ds
}

func (test *ResourceTest) TestListExecutions() {
	test.backend.On("ListExecutions", mock.Anything, mock.Anything).Return(&backtestPb.ListExecutionsResponse{}, nil)

	req := &grafana.CallResourceRequest{
		Method: http.MethodGet,
		Path:   "executions",
	}

	var r mockCallResourceResponseSender
	err := test.uut.CallResource(context.Background(), req, &r)
	test.Require().NoError(err, "fail to call resource")

	test.Equal(http.StatusOK, r.response.Status, "unexpected status")
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
}
