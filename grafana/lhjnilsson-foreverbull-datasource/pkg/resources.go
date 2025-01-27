package main

import (
	"encoding/json"
	"net/http"

	"github.com/grafana/grafana-plugin-sdk-go/backend/resource/httpadapter"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/pb"
)

type ResourceDefinition struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

func (ds *Datasource) registerResources() {
	httpMux := http.NewServeMux()
	httpMux.HandleFunc("/executions", ds.ListExecutions)
	httpMux.HandleFunc("/metrics", ds.ListMetrics)

	ds.CallResourceHandler = httpadapter.New(httpMux)
}

func (ds *Datasource) ListExecutions(w http.ResponseWriter, r *http.Request) {
	log := ds.log.With("method", "ListExecutions")

	rsp, err := ds.backend.ListExecutions(r.Context(), &pb.ListExecutionsRequest{})
	if err != nil {
		log.Error("fail to list executions", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	definitions := make([]ResourceDefinition, 0, len(rsp.Executions))
	for _, execution := range rsp.Executions {
		definitions = append(definitions, ResourceDefinition{
			Value: execution.Id,
			Label: execution.Id,
		})
	}

	data, err := json.Marshal(definitions)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	w.Write(data)
}

func (ds *Datasource) ListMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	w.Write([]byte(`[{"value": "metric1", "label": "metric1"}, {"value": "metric2", "label": "metric2"}]`))
}
