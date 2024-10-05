package command

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	common_pb "github.com/lhjnilsson/foreverbull/internal/pb"
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
		exists, err := ohlc.Exists(ctx, []string{symbol},
			common_pb.DateStringToDate(command.Start),
			common_pb.DateStringToDate(command.End))
		if err != nil {
			return fmt.Errorf("error checking OHLC existence: %w", err)
		}
		start, err := time.Parse("2006-01-02", command.Start)
		if err != nil {
			return fmt.Errorf("error parsing start date: %w", err)
		}
		end, err := time.Parse("2006-01-02", command.End)
		if err != nil {
			return fmt.Errorf("error parsing end date: %w", err)
		}

		if !exists {
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
	}
	return nil
}
