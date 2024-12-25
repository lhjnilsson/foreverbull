package main

import (
	"context"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

func TestQueryData(t *testing.T) {
	settings := backend.DataSourceInstanceSettings{}

	dsInstance, err := NewDatasource(context.Background(), settings)
	if err != nil {
		t.Error(err)
	}

	ds, isDataSource := dsInstance.(*Datasource)
	if !isDataSource {
		t.Fatal("Datasource must be an instance of Datasource")
	}

	resp, err := ds.QueryData(
		context.Background(),
		&backend.QueryDataRequest{
			Queries: []backend.DataQuery{
				{RefID: "A"},
			},
		},
	)
	if err != nil {
		t.Error(err)
	}

	if len(resp.Responses) != 1 {
		t.Fatal("QueryData must return a response")
	}
}
