package command

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/stream/dependency"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/marketdata"
	fs "github.com/lhjnilsson/foreverbull/pkg/finance/stream"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
)

func Ingest(ctx context.Context, message stream.Message) error {
	postgres, isDB := message.MustGet(stream.DBDep).(postgres.Query)
	if !isDB {
		return errors.New("db dependency casting failed")
	}

	yahoo, isMd := message.MustGet(dependency.MarketDataDep).(supplier.Marketdata)
	if !isMd {
		return errors.New("marketdata dependency casting failed")
	}

	command := fs.IngestCommand{}

	err := message.ParsePayload(&command)
	if err != nil {
		return fmt.Errorf("error unmarshalling OHLCIngest payload: %w", err)
	}

	assets := repository.Asset{Conn: postgres}
	ohlc := repository.OHLC{Conn: postgres}

	start, err := time.Parse("2006-01-02", command.Start)
	if err != nil {
		return fmt.Errorf("error parsing start date: %w", err)
	}

	var end *time.Time

	if command.End != nil {
		e, err := time.Parse("2006-01-02", *command.End)
		if err != nil {
			return fmt.Errorf("error parsing end date: %w", err)
		}

		end = &e
	}

	provider, err := marketdata.NormalizeProvider(command.Provider)
	if err != nil {
		return err
	}
	profilesFile := environment.GetTradingProfilesFile()
	if err := marketdata.ValidatePriceSource(provider, command.Profile, profilesFile); err != nil {
		return err
	}
	bars, err := marketdata.OHLCClient(provider, command.Profile, profilesFile, yahoo)
	if err != nil {
		return err
	}

	for _, symbol := range command.Symbols {
		_, err := assets.Get(ctx, symbol)
		if err != nil {
			if !errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("error getting asset: %w", err)
			}

			a, err := yahoo.GetAsset(symbol)
			if err != nil {
				return fmt.Errorf("error getting asset: %w", err)
			}

			err = assets.Store(ctx, a.Symbol, a.Name)
			if err != nil {
				return fmt.Errorf("error storing asset: %w", err)
			}
		}

		if err := ingestSymbol(ctx, &ohlc, bars, provider, symbol, start, end); err != nil {
			return err
		}
	}

	return nil
}

// rescaleTolerance is how far the stored and fetched close of the overlap day
// may differ before the stored history is treated as being on another scale,
// for example after a split.
const rescaleTolerance = 0.005

// ingestSymbol stores bars for one symbol.
//
// Nothing stored yet: the requested range is fetched and inserted.
//
// Stored history that starts on or before start: the fetch begins on the last
// stored day, so it overlaps one stored day. When that day comes back with a
// close within rescaleTolerance of the stored one, the new days are upserted.
// Otherwise (the close moved, the day is missing from the fetch, or either
// close is not a positive finite price) the scale of the stored history can
// not be trusted, so the whole range from the first stored day is fetched
// again and replaced.
//
// Stored history that starts after start: the fetch covers start through the
// last stored day and replaces it, so no stored day is left on an old scale.
func ingestSymbol(ctx context.Context, ohlc *repository.OHLC, bars supplier.Marketdata,
	provider, symbol string, start time.Time, end *time.Time,
) error {
	first, last, stored, err := ohlc.Range(ctx, provider, symbol)
	if err != nil {
		return err
	}

	if !stored {
		fetched, err := getOHLC(bars, symbol, start, end)
		if err != nil {
			return err
		}
		return storeBars(ctx, ohlc, provider, symbol, fetched)
	}

	if start.Before(first) {
		spanEnd := end
		if end != nil && end.Before(last) {
			spanEnd = &last
		}
		return replaceRange(ctx, ohlc, bars, provider, symbol, start, spanEnd)
	}

	if end != nil && !end.After(last) {
		// The requested range is already stored.
		return nil
	}

	fetched, err := getOHLC(bars, symbol, last, end)
	if err != nil {
		return err
	}

	matches, err := overlapMatches(ctx, ohlc, provider, symbol, last, fetched)
	if err != nil {
		return err
	}
	if !matches {
		return replaceRange(ctx, ohlc, bars, provider, symbol, first, end)
	}
	return storeBars(ctx, ohlc, provider, symbol, fetched)
}

// replaceRange fetches start through end and makes it the stored history for
// that span in one transaction. The refetched batch is validated first: if any
// bar is refused, the stored rows are left as they are and the
// InvalidPriceError is returned, so a bad bar never turns into a missing day.
func replaceRange(ctx context.Context, ohlc *repository.OHLC, bars supplier.Marketdata,
	provider, symbol string, start time.Time, end *time.Time,
) error {
	full, err := getOHLC(bars, symbol, start, end)
	if err != nil {
		return err
	}
	if err := repository.ValidateBars(provider, symbol, full); err != nil {
		return err
	}
	if err := ohlc.ReplaceRange(ctx, provider, symbol, full); err != nil {
		return fmt.Errorf("error replacing OHLC: %w", err)
	}
	return nil
}

// storeBars stores fetched in one transaction. StoreBars checks the whole
// batch with ValidateBars first, so a refused bar or a DB error stores none
// of them.
func storeBars(ctx context.Context, ohlc *repository.OHLC, provider, symbol string, fetched []*pb.OHLC) error {
	if err := ohlc.StoreBars(ctx, provider, symbol, fetched); err != nil {
		var invalid *repository.InvalidPriceError
		if errors.As(err, &invalid) {
			return err
		}
		return fmt.Errorf("error creating OHLC: %w", err)
	}
	return nil
}

func getOHLC(bars supplier.Marketdata, symbol string, start time.Time, end *time.Time) ([]*pb.OHLC, error) {
	ohlcs, err := bars.GetOHLC(symbol, start, end)
	if err != nil {
		// The blocked sentence already names the provider, symbol, and retry.
		if errors.Is(err, marketdata.ErrProviderBlocked) {
			return nil, err
		}
		return nil, fmt.Errorf("error getting OHLC: %w", err)
	}
	return ohlcs, nil
}

// overlapMatches reports whether the fetched close for the overlap day is a
// positive finite price within rescaleTolerance of the stored close. A missing
// overlap day or a bad close on either side is not a match, so the caller
// replaces the stored history instead of appending next to it.
func overlapMatches(ctx context.Context, ohlc *repository.OHLC, provider, symbol string, day time.Time, fetched []*pb.OHLC) (bool, error) {
	want := day.UTC().Format("2006-01-02")
	for _, o := range fetched {
		if o.GetTimestamp().AsTime().UTC().Format("2006-01-02") != want {
			continue
		}
		c := o.GetClose()
		if !(c > 0) || math.IsInf(c, 0) {
			return false, nil
		}
		storedClose, err := ohlc.Close(ctx, provider, symbol, day)
		if err != nil {
			return false, fmt.Errorf("error reading stored close: %w", err)
		}
		if !(storedClose > 0) || math.IsInf(storedClose, 0) {
			return false, nil
		}
		return math.Abs(c-storedClose)/storedClose <= rescaleTolerance, nil
	}
	return false, nil
}
