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

// ingestSymbol stores bars for one symbol. When the stored history already
// covers start, it fetches from the last stored day, so every incremental
// fetch overlaps one stored day. If that day's close moved by more than
// rescaleTolerance, the provider re-scaled its history, and the whole stored
// range is fetched again and replaced.
func ingestSymbol(ctx context.Context, ohlc *repository.OHLC, bars supplier.Marketdata,
	provider, symbol string, start time.Time, end *time.Time,
) error {
	first, last, stored, err := ohlc.Range(ctx, provider, symbol)
	if err != nil {
		return err
	}

	fetchStart := start
	incremental := stored && !first.After(start) && !last.Before(start)
	if incremental {
		if end != nil && !end.After(last) {
			// The requested range is already stored.
			return nil
		}
		fetchStart = last
	}

	fetched, err := getOHLC(bars, symbol, fetchStart, end)
	if err != nil {
		return err
	}

	if incremental && rescaled(ctx, ohlc, provider, symbol, last, fetched) {
		full, err := getOHLC(bars, symbol, first, end)
		if err != nil {
			return err
		}
		if err := ohlc.ReplaceRange(ctx, provider, symbol, full); err != nil {
			return fmt.Errorf("error replacing OHLC: %w", err)
		}
		return nil
	}

	// One transaction per symbol: a refused bar or a DB error stores none of them.
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

// rescaled reports whether the fetched close for the overlap day differs from
// the stored one by more than rescaleTolerance. A bad fetched close (zero or
// below, NaN or infinite) never counts as a re-scale.
func rescaled(ctx context.Context, ohlc *repository.OHLC, provider, symbol string, day time.Time, fetched []*pb.OHLC) bool {
	want := day.UTC().Format("2006-01-02")
	for _, o := range fetched {
		if o.GetTimestamp().AsTime().UTC().Format("2006-01-02") != want {
			continue
		}
		c := o.GetClose()
		if !(c > 0) || math.IsInf(c, 0) {
			return false
		}
		storedClose, err := ohlc.Close(ctx, provider, symbol, day)
		if err != nil || !(storedClose > 0) {
			return false
		}
		return math.Abs(c-storedClose)/storedClose > rescaleTolerance
	}
	return false
}
