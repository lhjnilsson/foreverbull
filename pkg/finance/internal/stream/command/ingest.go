package command

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/internal/redact"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/stream/dependency"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/marketdata"
	fs "github.com/lhjnilsson/foreverbull/pkg/finance/stream"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	"github.com/rs/zerolog/log"
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
				return logStoreError(&repository.StoreError{Symbol: symbol, Err: err}, "get asset")
			}

			a, err := yahoo.GetAsset(symbol)
			if err != nil {
				return fmt.Errorf("error getting asset: %w", redact.URLError(err))
			}

			// Outside the bars transaction: the asset row stays if the bars roll back.
			err = assets.Store(ctx, a.Symbol, a.Name)
			if err != nil {
				return logStoreError(&repository.StoreError{Symbol: symbol, Err: err}, "store asset")
			}
		}

		if err := ingestSymbol(ctx, &ohlc, bars, provider, symbol, start, end); err != nil {
			return err
		}
	}

	return nil
}

// ingestSymbol stores bars for one symbol through OHLC.Sync, the overlap and
// re-scale rule the DownloadHistoricalData RPC uses too. A database error
// becomes the fixed "Couldn't store prices for SYM. Try again …" sentence and
// its cause goes to the server log, so no SQL or driver text reaches the task
// state.
func ingestSymbol(ctx context.Context, ohlc *repository.OHLC, bars supplier.Marketdata,
	provider, symbol string, start time.Time, end *time.Time,
) error {
	err := ohlc.Sync(ctx, provider, symbol, start, end, func(from time.Time, to *time.Time) ([]*pb.OHLC, error) {
		return getOHLC(bars, symbol, from, to)
	})
	return logStoreError(err, "store bars")
}

// logStoreError logs the cause of a StoreError and returns it; its own text is
// the fixed sentence. Other errors come back unchanged.
func logStoreError(err error, step string) error {
	var stored *repository.StoreError
	if errors.As(err, &stored) {
		log.Error().Err(stored.Err).Str("symbol", stored.Symbol).Str("step", step).Msg("ingest failed to store prices")
	}
	return err
}

func getOHLC(bars supplier.Marketdata, symbol string, start time.Time, end *time.Time) ([]*pb.OHLC, error) {
	ohlcs, err := bars.GetOHLC(symbol, start, end)
	if err != nil {
		// The blocked sentence already names the provider, symbol, and retry.
		// It is checked before redact.URLError, which drops the chain.
		if errors.Is(err, marketdata.ErrProviderBlocked) {
			return nil, err
		}
		return nil, fmt.Errorf("error getting OHLC: %w", redact.URLError(err))
	}
	return ohlcs, nil
}
