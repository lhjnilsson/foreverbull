package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	grafana "github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	fbPb "github.com/lhjnilsson/foreverbull/pkg/pb"
	backtestpb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	backtestsvc "github.com/lhjnilsson/foreverbull/pkg/pb/backtest/service"
)

// executionLinkURL is filled per row from the execution UUID and backtest name.
// An empty name leaves var-backtest blank. The execution dashboard treats that
// as absent, so a link with only var-execution still opens the run.
const executionLinkURL = "/d/ee9ep4jxtwkcgf?var-execution=${__data.fields.execution}&var-backtest=${__data.fields.backtest}"

type overviewQuery struct {
	Backtest string `json:"backtest"`
}

type overviewRow struct {
	executionID           string
	backtest              string
	start                 string
	end                   string
	shortID               string
	algorithmPeriodReturn *float64
	maxDrawdown           *float64
	sharpe                *float64
	sortino               *float64
	portfolioValue        *float64
}

func (d *Datasource) HandleListExecutionOverview(ctx context.Context, req *grafana.QueryDataRequest) (*grafana.QueryDataResponse, error) {
	return processQueries(ctx, req, d.handleListExecutionOverview), nil
}

func (d *Datasource) handleListExecutionOverview(ctx context.Context, _ grafana.QueryDataRequest, q grafana.DataQuery) grafana.DataResponse {
	if d == nil || d.backend == nil || d.log == nil {
		return grafana.ErrDataResponse(grafana.StatusBadRequest, "broker client is not configured")
	}
	log := d.log.With("ListExecutionOverview")
	from, to := q.TimeRange.From, q.TimeRange.To
	if from.IsZero() || to.IsZero() {
		return grafana.ErrDataResponse(grafana.StatusBadRequest, "time range is required")
	}

	var model overviewQuery
	if len(q.JSON) > 0 {
		if err := json.Unmarshal(q.JSON, &model); err != nil {
			log.Error("fail to parse json", err)
			return grafana.ErrDataResponse(grafana.StatusBadRequest, fmt.Sprintf("json unmarshal: %v", err.Error()))
		}
	}
	backtestName := overviewBacktestName(model.Backtest)

	rsp, err := d.backend.ListExecutions(ctx, &backtestsvc.ListExecutionsRequest{Backtest: backtestName})
	if err != nil {
		log.Error("fail to list executions", err)
		return grafana.ErrDataResponse(grafana.StatusBadRequest, fmt.Sprintf("list executions: %v", err.Error()))
	}

	rows := make([]overviewRow, 0)
	for _, execution := range rsp.GetExecutions() {
		if execution == nil || execution.GetId() == "" {
			continue
		}
		if backtestName != "" && execution.GetBacktest() != backtestName {
			continue
		}
		// Overlap is the execution's start_date..end_date, not period timestamps.
		if !executionOverlaps(execution.GetStartDate(), execution.GetEndDate(), from, to) {
			continue
		}
		got, err := d.backend.GetExecution(ctx, &backtestsvc.GetExecutionRequest{ExecutionId: execution.GetId()})
		if err != nil {
			log.Error("fail to get execution", err)
			return grafana.ErrDataResponse(grafana.StatusBadRequest, fmt.Sprintf("get execution: %v", err.Error()))
		}
		rows = append(rows, overviewRowFrom(execution, got.GetPeriods()))
	}
	sortOverviewRows(rows)

	frame := overviewFrame(q.RefID, rows)
	return grafana.DataResponse{
		Frames: []*data.Frame{frame},
		Status: grafana.StatusOK,
	}
}

// overviewBacktestName is empty when the dashboard variable means every backtest.
// "*" is the All value (a truthy allValue). "$__all" and an uninterpolated
// "$backtest" are the same choice.
func overviewBacktestName(raw string) string {
	name := strings.TrimSpace(raw)
	switch name {
	case "", "*", "$__all", "$backtest":
		return ""
	default:
		return name
	}
}

// executionOverlaps reports whether inclusive UTC calendar dates
// start_date..end_date overlap the picker [from, to].
// An execution that ends on the picker's from date is included.
// One that ends the day before is not. Times of day do not shrink the date.
func executionOverlaps(start, end *fbPb.Date, from, to time.Time) bool {
	startDay, okStart := utcProtoDate(start)
	endDay, okEnd := utcProtoDate(end)
	if !okStart || !okEnd || from.IsZero() || to.IsZero() {
		return false
	}
	if endDay.Before(startDay) {
		startDay, endDay = endDay, startDay
	}
	fromDay := utcDay(from)
	toDay := utcDay(to)
	if toDay.Before(fromDay) {
		fromDay, toDay = toDay, fromDay
	}
	return !startDay.After(toDay) && !endDay.Before(fromDay)
}

func utcDay(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

func utcProtoDate(d *fbPb.Date) (time.Time, bool) {
	if d == nil || d.GetYear() == 0 {
		return time.Time{}, false
	}
	return time.Date(int(d.GetYear()), time.Month(d.GetMonth()), int(d.GetDay()), 0, 0, 0, 0, time.UTC), true
}

// lastStoredPeriod is the period with the latest date.
// A nil Sharpe on that period stays nil; earlier periods are not borrowed.
// ListExecutions Result is not used: that join keeps one global period.
func lastStoredPeriod(periods []*backtestpb.Period) *backtestpb.Period {
	var best *backtestpb.Period
	var bestAt time.Time
	bestIndex := -1
	for i, period := range periods {
		if period == nil {
			continue
		}
		at := periodTime(period)
		if best == nil || at.After(bestAt) || (at.Equal(bestAt) && i > bestIndex) {
			best = period
			bestAt = at
			bestIndex = i
		}
	}
	return best
}

func overviewRowFrom(execution *backtestpb.Execution, periods []*backtestpb.Period) overviewRow {
	row := overviewRow{
		executionID: execution.GetId(),
		backtest:    execution.GetBacktest(),
		start:       formatDate(execution.GetStartDate()),
		end:         formatDate(execution.GetEndDate()),
		shortID:     shortExecutionID(execution.GetId()),
	}
	period := lastStoredPeriod(periods)
	if period == nil {
		return row
	}
	// Stored numbers. percentunit / currencyUSD / none are display units only.
	row.algorithmPeriodReturn = newFloat(period.GetAlgorithmPeriodReturn())
	row.maxDrawdown = newFloat(period.GetMaxDrawdown())
	row.portfolioValue = newFloat(period.GetPortfolioValue())
	row.sharpe = cloneFloat(period.Sharpe)
	row.sortino = cloneFloat(period.Sortino)
	return row
}

func newFloat(v float64) *float64 {
	copied := v
	return &copied
}

func cloneFloat(v *float64) *float64 {
	if v == nil {
		return nil
	}
	return newFloat(*v)
}

func sortOverviewRows(rows []overviewRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		left, right := rows[i], rows[j]
		switch {
		case left.sharpe == nil && right.sharpe == nil:
			return overviewLess(left, right)
		case left.sharpe == nil:
			return false
		case right.sharpe == nil:
			return true
		case *left.sharpe != *right.sharpe:
			return *left.sharpe > *right.sharpe
		default:
			return overviewLess(left, right)
		}
	})
}

func overviewLess(left, right overviewRow) bool {
	if left.backtest != right.backtest {
		return left.backtest < right.backtest
	}
	return left.executionID < right.executionID
}

func overviewFrame(refID string, rows []overviewRow) *data.Frame {
	n := len(rows)
	backtests := make([]string, n)
	starts := make([]string, n)
	ends := make([]string, n)
	ids := make([]string, n)
	executions := make([]string, n)
	returns := make([]*float64, n)
	drawdowns := make([]*float64, n)
	sharpes := make([]*float64, n)
	sortinos := make([]*float64, n)
	values := make([]*float64, n)
	for i, row := range rows {
		backtests[i] = row.backtest
		starts[i] = row.start
		ends[i] = row.end
		ids[i] = row.shortID
		executions[i] = row.executionID
		returns[i] = row.algorithmPeriodReturn
		drawdowns[i] = row.maxDrawdown
		sharpes[i] = row.sharpe
		sortinos[i] = row.sortino
		values[i] = row.portfolioValue
	}

	frame := data.NewFrame("Executions",
		overviewTextField("backtest", "Backtest", backtests, true),
		overviewTextField("start", "Start", starts, true),
		overviewTextField("end", "End", ends, true),
		overviewTextField("id", "ID", ids, true),
		overviewNumberField("algorithm_period_return", "Algorithm period return", "percentunit", returns),
		overviewNumberField("max_drawdown", "Max drawdown", "percentunit", drawdowns),
		overviewNumberField("sharpe", "Sharpe", "none", sharpes),
		overviewNumberField("sortino", "Sortino", "none", sortinos),
		overviewNumberField("portfolio_value", "Ending portfolio value", "currencyUSD", values),
		overviewHiddenText("execution", "Execution", executions),
	)
	frame.RefID = refID
	frame.Meta = &data.FrameMeta{Type: data.FrameTypeTable}
	return frame
}

func overviewLinks() []data.DataLink {
	return []data.DataLink{{
		Title: "Open execution",
		URL:   executionLinkURL,
	}}
}

func overviewTextField(name, display string, values []string, link bool) *data.Field {
	field := data.NewField(name, nil, values)
	cfg := &data.FieldConfig{DisplayNameFromDS: display}
	if link {
		cfg.Links = overviewLinks()
	}
	field.SetConfig(cfg)
	return field
}

func overviewHiddenText(name, display string, values []string) *data.Field {
	field := data.NewField(name, nil, values)
	field.SetConfig(&data.FieldConfig{
		DisplayNameFromDS: display,
		Custom:            map[string]interface{}{"hidden": true},
	})
	return field
}

func overviewNumberField(name, display, unit string, values []*float64) *data.Field {
	field := data.NewField(name, nil, values)
	field.SetConfig(&data.FieldConfig{
		DisplayNameFromDS: display,
		Unit:              unit,
	})
	return field
}
