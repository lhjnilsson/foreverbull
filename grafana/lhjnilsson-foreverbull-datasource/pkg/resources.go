package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend/resource/httpadapter"
	fbPb "github.com/lhjnilsson/foreverbull/pkg/pb"
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

func formatDate(d *fbPb.Date) string {
	if d == nil || d.Year == 0 {
		return ""
	}
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day)
}

func shortExecutionID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// executionLabel is the human picker text. Value stays the execution UUID.
// Format: "{backtest} {start}–{end} {short id}"
func executionLabel(execution *backtestPb.Execution) string {
	if execution == nil {
		return ""
	}
	parts := make([]string, 0, 3)
	if execution.Backtest != "" {
		parts = append(parts, execution.Backtest)
	}
	start := formatDate(execution.StartDate)
	end := formatDate(execution.EndDate)
	switch {
	case start != "" && end != "":
		parts = append(parts, start+"–"+end)
	case start != "":
		parts = append(parts, start)
	case end != "":
		parts = append(parts, end)
	}
	if short := shortExecutionID(execution.Id); short != "" {
		parts = append(parts, short)
	}
	return strings.Join(parts, " ")
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
			Label: executionLabel(execution),
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
	definitions := make([]ResourceDefinition, 0, len(listedMetrics))
	for _, metric := range listedMetrics {
		definitions = append(definitions, ResourceDefinition{
			Value: string(metric),
			Label: string(metric),
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
