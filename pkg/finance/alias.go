package finance

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
)

// ErrAmbiguousSymbol is a ticker asset_symbol gives to more than one
// instrument on the dates asked. Callers refuse it. They do not pick one.
var ErrAmbiguousSymbol = repository.ErrAmbiguousSymbol

// SymbolHasInstrument returns the ISIN asset_symbol gives symbol on a date
// of [start, end]. A nil end is open. None is an empty ISIN and a nil
// error. More than one instrument is ErrAmbiguousSymbol. A nil database is
// an error, not "no instrument".
func SymbolHasInstrument(ctx context.Context, conn postgres.Query, symbol string, start time.Time, end *time.Time) (string, error) {
	if conn == nil {
		return "", errors.New("no database to read ticker aliases")
	}
	inst, err := (&repository.Instruments{Conn: conn}).Overlapping(ctx, symbol, start, end)
	if err == nil {
		return inst.ISIN, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return "", fmt.Errorf("read instrument for %s: %w", symbol, err)
}
