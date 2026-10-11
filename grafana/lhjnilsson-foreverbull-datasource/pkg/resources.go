package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend/resource/httpadapter"
	fbPb "github.com/lhjnilsson/foreverbull/pkg/pb"
	backtestPb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	backtestsvc "github.com/lhjnilsson/foreverbull/pkg/pb/backtest/service"
	strategyPb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy"
	strategysvc "github.com/lhjnilsson/foreverbull/pkg/pb/strategy/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ResourceDefinition struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	// Start and End are YYYY-MM-DD from ListExecutions. The time axis uses them
	// before period frames return. Other resources leave them empty.
	Start string `json:"start,omitempty"`
	End   string `json:"end,omitempty"`
}

// resourceFilter is accepted as query params or a JSON POST body.
// Grafana variables cascade strategy → version → experiment/backtest → execution.
type resourceFilter struct {
	Backtest     string `json:"backtest"`
	SessionID    string `json:"session_id"`
	StrategyName string `json:"strategy"`
	VersionID    string `json:"version"`
	BacktestName string `json:"backtest_name"`
	ExperimentID string `json:"experiment"`
}

func (ds *Datasource) registerResources() {
	httpMux := http.NewServeMux()
	httpMux.HandleFunc("/executions", ds.ListExecutions)
	httpMux.HandleFunc("/execution", ds.GetExecutionArguments)
	httpMux.HandleFunc("/backtests", ds.ListBacktests)
	httpMux.HandleFunc("/metrics", ds.ListMetrics)
	httpMux.HandleFunc("/strategies", ds.ListStrategies)
	httpMux.HandleFunc("/versions", ds.ListVersions)
	httpMux.HandleFunc("/experiments", ds.ListExperiments)

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

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func parseResourceFilter(r *http.Request) resourceFilter {
	var f resourceFilter
	if r == nil {
		return f
	}
	q := r.URL.Query()
	f.Backtest = firstNonEmpty(q.Get("backtest"), q.Get("backtest_name"))
	f.SessionID = firstNonEmpty(q.Get("session_id"), q.Get("session"))
	f.StrategyName = firstNonEmpty(q.Get("strategy"), q.Get("strategy_name"))
	f.VersionID = firstNonEmpty(q.Get("version"), q.Get("version_id"))
	f.BacktestName = firstNonEmpty(q.Get("backtest_name"), f.Backtest)
	f.ExperimentID = firstNonEmpty(q.Get("experiment"), q.Get("experiment_id"))
	if r.Body != nil {
		body, err := io.ReadAll(r.Body)
		if err == nil && len(body) > 0 {
			var posted resourceFilter
			if json.Unmarshal(body, &posted) == nil {
				f.Backtest = firstNonEmpty(posted.Backtest, posted.BacktestName, f.Backtest)
				f.SessionID = firstNonEmpty(posted.SessionID, f.SessionID)
				f.StrategyName = firstNonEmpty(posted.StrategyName, f.StrategyName)
				f.VersionID = firstNonEmpty(posted.VersionID, f.VersionID)
				f.BacktestName = firstNonEmpty(posted.BacktestName, posted.Backtest, f.BacktestName)
				f.ExperimentID = firstNonEmpty(posted.ExperimentID, f.ExperimentID)
			}
		}
	}
	if f.BacktestName == "" {
		f.BacktestName = f.Backtest
	}
	return f
}

func writeJSON(w http.ResponseWriter, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Add("Content-Type", "application/json")
	_, _ = w.Write(data)
}

func strategyModuleMissing(err error) bool {
	return status.Code(err) == codes.Unimplemented
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

// experimentLabel is strategy / rev / backtest. Chart value is still execution_id.
func experimentLabel(exp *strategyPb.Experiment) string {
	if exp == nil {
		return ""
	}
	parts := make([]string, 0, 3)
	if exp.StrategyName != "" {
		parts = append(parts, exp.StrategyName)
	}
	if exp.VersionRev != 0 {
		parts = append(parts, fmt.Sprintf("rev %d", exp.VersionRev))
	} else if exp.VersionId != "" {
		parts = append(parts, shortExecutionID(exp.VersionId))
	}
	if exp.BacktestName != "" {
		parts = append(parts, exp.BacktestName)
	}
	return strings.Join(parts, " / ")
}

func versionLabel(exp *strategyPb.Experiment) string {
	if exp == nil {
		return ""
	}
	if exp.VersionRev != 0 {
		return fmt.Sprintf("rev %d", exp.VersionRev)
	}
	return shortExecutionID(exp.VersionId)
}

func (ds *Datasource) ListExecutions(w http.ResponseWriter, r *http.Request) {
	log := ds.log.With("method", "ListExecutions")
	filter := parseResourceFilter(r)

	allowed := map[string]struct{}{}
	if filter.StrategyName != "" || filter.VersionID != "" || filter.ExperimentID != "" {
		ids, err := ds.experimentExecutionIDs(r, filter)
		if err != nil {
			if strategyModuleMissing(err) {
				writeJSON(w, []ResourceDefinition{})
				return
			}
			log.Error("fail to list experiments for execution filter", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if len(ids) == 0 {
			writeJSON(w, []ResourceDefinition{})
			return
		}
		allowed = ids
	}

	rsp, err := ds.backend.ListExecutions(r.Context(), &backtestsvc.ListExecutionsRequest{
		Backtest:  firstNonEmpty(filter.Backtest, filter.BacktestName),
		SessionId: filter.SessionID,
	})
	if err != nil {
		log.Error("fail to list executions", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	definitions := make([]ResourceDefinition, 0, len(rsp.GetExecutions()))
	for _, execution := range rsp.GetExecutions() {
		if len(allowed) > 0 {
			if _, ok := allowed[execution.GetId()]; !ok {
				continue
			}
		}
		definitions = append(definitions, ResourceDefinition{
			Value: execution.Id,
			Label: executionLabel(execution),
			Start: formatDate(execution.StartDate),
			End:   formatDate(execution.EndDate),
		})
	}

	writeJSON(w, definitions)
}

type executionParameterView struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	ValueType string `json:"value_type"`
}

type executionFunctionView struct {
	Name       string                   `json:"name"`
	Parameters []executionParameterView `json:"parameters"`
}

// executionArgumentsView is one run's stored worker arguments.
// It does not include database URL, broker port, or namespace port.
type executionArgumentsView struct {
	ID        string                  `json:"id"`
	Session   string                  `json:"session"`
	Backtest  string                  `json:"backtest"`
	Benchmark string                  `json:"benchmark"`
	Start     string                  `json:"start"`
	End       string                  `json:"end"`
	Functions []executionFunctionView `json:"functions"`
}

type executionArgumentsRequest struct {
	ID        string `json:"id"`
	Execution string `json:"execution"`
}

func executionArgumentsID(r *http.Request) string {
	if r == nil {
		return ""
	}
	q := r.URL.Query()
	id := firstNonEmpty(q.Get("id"), q.Get("execution"))
	if r.Body == nil {
		return id
	}
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		return id
	}
	var posted executionArgumentsRequest
	if json.Unmarshal(body, &posted) != nil {
		return id
	}
	return firstNonEmpty(posted.ID, posted.Execution, id)
}

func (ds *Datasource) ListBacktests(w http.ResponseWriter, r *http.Request) {
	log := ds.log.With("method", "ListBacktests")
	rsp, err := ds.backend.ListBacktests(r.Context(), &backtestsvc.ListBacktestsRequest{})
	if err != nil {
		log.Error("fail to list backtests", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	definitions := make([]ResourceDefinition, 0, len(rsp.GetBacktests()))
	for _, bt := range rsp.GetBacktests() {
		name := bt.GetName()
		if name == "" {
			continue
		}
		definitions = append(definitions, ResourceDefinition{
			Value: name,
			Label: name,
		})
	}
	writeJSON(w, definitions)
}

func (ds *Datasource) GetExecutionArguments(w http.ResponseWriter, r *http.Request) {
	log := ds.log.With("method", "GetExecutionArguments")
	id := executionArgumentsID(r)
	if id == "" {
		http.Error(w, "execution id is required", http.StatusBadRequest)
		return
	}

	rsp, err := ds.backend.GetExecution(r.Context(), &backtestsvc.GetExecutionRequest{ExecutionId: id})
	if err != nil {
		log.Error("fail to get execution", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	execution := rsp.GetExecution()
	if execution == nil {
		http.Error(w, "execution not found", http.StatusInternalServerError)
		return
	}

	functions := make([]executionFunctionView, 0, len(rsp.GetFunctions()))
	for _, fn := range rsp.GetFunctions() {
		if fn == nil {
			continue
		}
		params := make([]executionParameterView, 0, len(fn.GetParameters()))
		for _, p := range fn.GetParameters() {
			if p == nil {
				continue
			}
			params = append(params, executionParameterView{
				Key:       p.GetKey(),
				Value:     p.GetValue(),
				ValueType: p.GetValueType(),
			})
		}
		functions = append(functions, executionFunctionView{
			Name:       fn.GetName(),
			Parameters: params,
		})
	}

	writeJSON(w, executionArgumentsView{
		ID:        execution.GetId(),
		Session:   execution.GetSession(),
		Backtest:  execution.GetBacktest(),
		Benchmark: ds.executionBenchmark(r, execution.GetBacktest()),
		Start:     formatDate(execution.GetStartDate()),
		End:       formatDate(execution.GetEndDate()),
		Functions: functions,
	})
}

// executionBenchmark reads the backtest row. A lookup failure leaves it empty.
// Database URL and ports are not part of this view.
func (ds *Datasource) executionBenchmark(r *http.Request, name string) string {
	if ds == nil || ds.backend == nil || r == nil || name == "" {
		return ""
	}
	rsp, err := ds.backend.GetBacktest(r.Context(), &backtestsvc.GetBacktestRequest{Name: name})
	if err != nil || rsp == nil {
		if err != nil && ds.log != nil {
			ds.log.With("method", "GetExecutionArguments").Error("fail to get backtest", err)
		}
		return ""
	}
	if rsp.GetBacktest() == nil {
		return ""
	}
	return rsp.GetBacktest().GetBenchmark()
}

func (ds *Datasource) ListMetrics(w http.ResponseWriter, r *http.Request) {
	definitions := make([]ResourceDefinition, 0, len(listedMetrics))
	for _, metric := range listedMetrics {
		definitions = append(definitions, ResourceDefinition{
			Value: string(metric),
			Label: string(metric),
		})
	}
	writeJSON(w, definitions)
}

func (ds *Datasource) ListStrategies(w http.ResponseWriter, r *http.Request) {
	log := ds.log.With("method", "ListStrategies")
	if ds.strategy == nil {
		writeJSON(w, []ResourceDefinition{})
		return
	}

	rsp, err := ds.strategy.ListStrategies(r.Context(), &strategysvc.ListStrategiesRequest{})
	if err != nil {
		if strategyModuleMissing(err) {
			writeJSON(w, []ResourceDefinition{})
			return
		}
		log.Error("fail to list strategies", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	definitions := make([]ResourceDefinition, 0, len(rsp.GetStrategies()))
	for _, strategy := range rsp.GetStrategies() {
		name := strategy.GetName()
		definitions = append(definitions, ResourceDefinition{
			Value:       name,
			Label:       name,
			Description: strategy.GetId(),
		})
	}
	writeJSON(w, definitions)
}

func (ds *Datasource) ListVersions(w http.ResponseWriter, r *http.Request) {
	log := ds.log.With("method", "ListVersions")
	filter := parseResourceFilter(r)
	experiments, err := ds.listExperiments(r, filter)
	if err != nil {
		if strategyModuleMissing(err) {
			writeJSON(w, []ResourceDefinition{})
			return
		}
		log.Error("fail to list versions", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	seen := map[string]struct{}{}
	definitions := make([]ResourceDefinition, 0)
	for _, exp := range experiments {
		id := exp.GetVersionId()
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		definitions = append(definitions, ResourceDefinition{
			Value:       id,
			Label:       versionLabel(exp),
			Description: exp.GetStrategyName(),
		})
	}
	writeJSON(w, definitions)
}

func (ds *Datasource) ListExperiments(w http.ResponseWriter, r *http.Request) {
	log := ds.log.With("method", "ListExperiments")
	filter := parseResourceFilter(r)
	experiments, err := ds.listExperiments(r, filter)
	if err != nil {
		if strategyModuleMissing(err) {
			writeJSON(w, []ResourceDefinition{})
			return
		}
		log.Error("fail to list experiments", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	definitions := make([]ResourceDefinition, 0, len(experiments))
	for _, exp := range experiments {
		if filter.ExperimentID != "" && exp.GetId() != filter.ExperimentID {
			continue
		}
		executionID := exp.GetExecutionId()
		if executionID == "" {
			continue
		}
		definitions = append(definitions, ResourceDefinition{
			Value:       executionID,
			Label:       experimentLabel(exp),
			Description: exp.GetId(),
		})
	}
	writeJSON(w, definitions)
}

func (ds *Datasource) listExperiments(r *http.Request, filter resourceFilter) ([]*strategyPb.Experiment, error) {
	if ds.strategy == nil {
		return nil, status.Error(codes.Unimplemented, "strategy module is not registered")
	}
	rsp, err := ds.strategy.ListExperiments(r.Context(), &strategysvc.ListExperimentsRequest{
		StrategyName: filter.StrategyName,
		VersionId:    filter.VersionID,
		BacktestName: filter.BacktestName,
	})
	if err != nil {
		return nil, err
	}
	return rsp.GetExperiments(), nil
}

func (ds *Datasource) experimentExecutionIDs(r *http.Request, filter resourceFilter) (map[string]struct{}, error) {
	experiments, err := ds.listExperiments(r, filter)
	if err != nil {
		return nil, err
	}
	ids := make(map[string]struct{}, len(experiments))
	for _, exp := range experiments {
		if filter.ExperimentID != "" && exp.GetId() != filter.ExperimentID {
			continue
		}
		if id := exp.GetExecutionId(); id != "" {
			ids[id] = struct{}{}
		}
	}
	return ids, nil
}
