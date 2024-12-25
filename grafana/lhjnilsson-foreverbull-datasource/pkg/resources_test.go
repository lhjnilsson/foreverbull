package main

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

type mockCallResourceResponseSender struct {
	response *backend.CallResourceResponse
}

func (s *mockCallResourceResponseSender) Send(response *backend.CallResourceResponse) error {
	s.response = response
	return nil
}

func TestListExecutions(t *testing.T) {
	settings := backend.DataSourceInstanceSettings{}

	dsInstance, err := NewDatasource(context.Background(), settings)
	if err != nil {
		t.Error(err)
	}

	ds, isDataSource := dsInstance.(*Datasource)
	if !isDataSource {
		t.Fatal("Datasource must be an instance of Datasource")
	}

	req := &backend.CallResourceRequest{
		Method: http.MethodGet,
		Path:   "executions",
	}
	var r mockCallResourceResponseSender
	err = ds.CallResource(context.Background(), req, &r)
	if err != nil {
		t.Error(err)
	}

	fmt.Println(string(r.response.Body))
}

func TestListMetrics(t *testing.T) {
	settings := backend.DataSourceInstanceSettings{}

	dsInstance, err := NewDatasource(context.Background(), settings)
	if err != nil {
		t.Error(err)
	}

	ds, isDataSource := dsInstance.(*Datasource)
	if !isDataSource {
		t.Fatal("Datasource must be an instance of Datasource")
	}

	req := &backend.CallResourceRequest{
		Method: http.MethodGet,
		Path:   "metrics",
	}
	var r mockCallResourceResponseSender
	err = ds.CallResource(context.Background(), req, &r)
	if err != nil {
		t.Error(err)
	}

	fmt.Println(string(r.response.Body))
}
