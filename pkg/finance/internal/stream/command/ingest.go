package command

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/stream/dependency"
	fs "github.com/lhjnilsson/foreverbull/pkg/finance/stream"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
)

func Ingest(ctx context.Context, message stream.Message) error {
	db := message.MustGet(stream.DBDep).(postgres.Query)
	marketdata := message.MustGet(dependency.MarketDataDep).(supplier.Marketdata)

	command := fs.IngestCommand{}
	err := message.ParsePayload(&command)
	if err != nil {
		return fmt.Errorf("error unmarshalling OHLCIngest payload: %w", err)
	}

	assets := repository.Asset{Conn: db}
	ohlc := repository.OHLC{Conn: db}

	for _, symbol := range command.Symbols {
		_, err := assets.Get(ctx, symbol)
		if err != nil {
			if err != pgx.ErrNoRows {
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
		exists, err := ohlc.Exists(ctx, []string{symbol}, command.Start, command.End)
		if err != nil {
			return fmt.Errorf("error checking OHLC existence: %w", err)
		}
		if !exists {
			ohlcs, err := marketdata.GetOHLC(symbol, command.Start, command.End)
			if err != nil {
				return fmt.Errorf("error getting OHLC: %w", err)
			}
			for _, o := range *ohlcs {
				err = ohlc.Store(ctx, symbol, o.Time, o.Open, o.High, o.Low, o.Close, o.Volume)
				if err != nil {
					return fmt.Errorf("error creating OHLC: %w", err)
				}
			}
		}
	}
	return nil
}
