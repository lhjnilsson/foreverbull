// Package bars loads alias-resolved OHLC for a strategy worker.
// The worker receives the bars on WorkerRequest and does not open Postgres.
package bars

import (
	"context"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	financepb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
)

// Source reads stored bars for the symbols an algorithm asked for.
type Source struct {
	ohlc repository.OHLC
}

// New reads bars through conn. conn is the broker's database handle.
func New(conn postgres.Query) *Source {
	return &Source{ohlc: repository.OHLC{Conn: conn}}
}

// List returns the lookback the worker used to query itself.
func (s *Source) List(ctx context.Context, provider string, symbols []string, asOf time.Time, limit int) ([]*financepb.OHLC, error) {
	return s.ohlc.ListResolved(ctx, provider, symbols, asOf, limit)
}
