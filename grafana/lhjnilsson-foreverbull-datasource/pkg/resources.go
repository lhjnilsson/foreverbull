package main

import (
	"encoding/json"
	"net/http"

	"github.com/grafana/grafana-plugin-sdk-go/backend/resource/httpadapter"
	backtestPb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
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

	rsp, err := ds.backend.ListExecutions(r.Context(), &backtestPb.ListExecutionsRequest{})
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
	definitions := []ResourceDefinition{
		{Value: string(PortfolioValue), Label: string(PortfolioValue)},
		{Value: string(Alpha), Label: string(Alpha)},
		{Value: string(Beta), Label: string(Beta)},
		{Value: string(Sharpe), Label: string(Sharpe)},
		{Value: string(Sortino), Label: string(Sortino)},
		{Value: string(CapitalUsed), Label: string(CapitalUsed)},
		{Value: string(LongCount), Label: string(LongCount)},
		{Value: string(ShortCount), Label: string(ShortCount)},
		{Value: string(LongValue), Label: string(LongValue)},
		{Value: string(ShortValue), Label: string(ShortValue)},
	}

	data, err := json.Marshal(definitions)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	w.Write(data)
}
