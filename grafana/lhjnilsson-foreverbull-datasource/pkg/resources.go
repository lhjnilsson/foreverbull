package main

import (
	"fmt"
	"net/http"

	"github.com/grafana/grafana-plugin-sdk-go/backend/resource/httpadapter"
)

func (ds *Datasource) registerResources() {
	httpMux := http.NewServeMux()
	httpMux.HandleFunc("/executions", ds.ListExecutions)
	httpMux.HandleFunc("/metrics", ds.ListMetrics)

	fmt.Println("Registering resources")
	ds.CallResourceHandler = httpadapter.New(httpMux)
}

func (ds *Datasource) ListExecutions(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	w.Write([]byte(`[{"value": "execution1", "label": "exc2"}, {"value": "execution2", "label": "exc222"}]`))
}

func (ds *Datasource) ListMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	w.Write([]byte(`[{"value": "metric1", "label": "metric1"}, {"value": "metric2", "label": "metric2"}]`))
}
