package api

import (
	"fmt"

	commonPB "github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// resolveRunDates checks a run's dates against the backtest's periods before
// the execution is stored. A run outside them has no universe and no bars, so
// it would end flat instead of failing. A missing date takes the backtest's
// first or last day. A backtest without periods has no data at all, so any run
// of it is refused before an execution is stored.
func resolveRunDates(stored *backtest.Backtest, start, end *commonPB.Date) (*commonPB.Date, *commonPB.Date, error) {
	first, last := backtestRange(stored.GetPeriods())
	if first == nil || last == nil {
		return nil, nil, status.Errorf(codes.FailedPrecondition,
			"Backtest '%[1]s' has no data to run. Delete it with `fbull backtest delete %[1]s`, "+
				"then create it again with `fbull backtest create`.", stored.GetName())
	}
	if start == nil {
		start = first
	}
	if end == nil {
		end = last
	}

	name := stored.GetName()
	firstDay, lastDay := commonPB.DateToDateString(first), commonPB.DateToDateString(last)
	hint := fmt.Sprintf("Pick dates between %s and %s, or leave out the dates to run the whole backtest.", firstDay, lastDay)
	startTime, endTime := commonPB.DateToTime(start), commonPB.DateToTime(end)

	switch {
	case startTime.Before(commonPB.DateToTime(first)):
		return nil, nil, status.Errorf(codes.InvalidArgument,
			"Start %s is before backtest '%s' begins. %s", commonPB.DateToDateString(start), name, hint)
	case endTime.After(commonPB.DateToTime(last)):
		return nil, nil, status.Errorf(codes.InvalidArgument,
			"End %s is after backtest '%s' ends. %s", commonPB.DateToDateString(end), name, hint)
	case startTime.After(endTime):
		return nil, nil, status.Errorf(codes.InvalidArgument,
			"Start %s is after end %s. Pick a start on or before the end.",
			commonPB.DateToDateString(start), commonPB.DateToDateString(end))
	}
	return start, end, nil
}

// backtestRange returns the earliest period start and the latest period end.
func backtestRange(periods []*backtest.Backtest_Period) (*commonPB.Date, *commonPB.Date) {
	var first, last *commonPB.Date
	for _, period := range periods {
		if period.GetStartDate() == nil || period.GetEndDate() == nil {
			continue
		}
		if first == nil || commonPB.DateToTime(period.GetStartDate()).Before(commonPB.DateToTime(first)) {
			first = period.GetStartDate()
		}
		if last == nil || commonPB.DateToTime(period.GetEndDate()).After(commonPB.DateToTime(last)) {
			last = period.GetEndDate()
		}
	}
	return first, last
}
