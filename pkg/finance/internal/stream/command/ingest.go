package command

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/stream/dependency"
	fs "github.com/lhjnilsson/foreverbull/pkg/finance/stream"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
)

func Ingest(ctx context.Context, message stream.Message) error {
	postgres, isDB := message.MustGet(stream.DBDep).(postgres.Query)
	if !isDB {
		return errors.New("db dependency casting failed")
	}

	marketdata, isMd := message.MustGet(dependency.MarketDataDep).(supplier.Marketdata)
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

	for _, symbol := range command.Symbols {
		_, err := assets.Get(ctx, symbol)
		if err != nil {
			if !errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("error getting asset: %w", err)
			}

			a, err := marketdata.GetAsset(symbol)
			if err != nil {
				return fmt.Errorf("error getting asset: %w", err)
			}

			err = assets.Store(ctx, a.Symbol, a.Name)
			if err != nil {
				return fmt.Errorf("error storing asset: %w", err)
			}
		}

		ohlcs, err := marketdata.GetOHLC(symbol, start, end)
		if err != nil {
			return fmt.Errorf("error getting OHLC: %w", err)
		}

		for _, o := range ohlcs {
			err = ohlc.Store(ctx, symbol, o.Timestamp.AsTime(), o.Open, o.High, o.Low, o.Close, int(o.Volume))
			if err != nil {
				return fmt.Errorf("error creating OHLC: %w", err)
			}
		}
	}

	return nil
}
