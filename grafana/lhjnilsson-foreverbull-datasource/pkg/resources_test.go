package main

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	grafana "github.com/grafana/grafana-plugin-sdk-go/backend"
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

	uut *Datasource
}

func (test *ResourceTest) SetupTest() {
	fmt.Println("SETUP")

	settings := grafana.DataSourceInstanceSettings{}

	dsInstance, err := NewDatasource(context.Background(), settings)
	test.Require().NoError(err, "fail to create datasource")

	ds, isDataSource := dsInstance.(*Datasource)
	test.Require().True(isDataSource, "Datasource must be an instance of Datasource")

	test.uut = ds
}

func (test *ResourceTest) TestListExecutions() {
	req := &grafana.CallResourceRequest{
		Method: http.MethodGet,
		Path:   "executions",
	}

	var r mockCallResourceResponseSender
	err := test.uut.CallResource(context.Background(), req, &r)
	test.Require().NoError(err, "fail to call resource")
}
