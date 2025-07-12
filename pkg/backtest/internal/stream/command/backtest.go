package command

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/dependency"
	ss "github.com/lhjnilsson/foreverbull/pkg/backtest/stream"
	"github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest/service"
	financeServicePB "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
)

func CreateBacktest(ctx context.Context, msg stream.Message) error {
	command := ss.CreateBacktestCommand{}

	err := msg.ParsePayload(&command)
	if err != nil {
		return fmt.Errorf("error unmarshalling CreateBacktest payload: %w", err)
	}
	// Get backtest
	db := msg.MustGet(stream.DBDep).(*pgxpool.Pool)
	backtests := repository.Backtest{Conn: db}
	b, err := backtests.Get(ctx, command.BacktestName)
	if err != nil {
		return fmt.Errorf("error getting backtest: %w", err)
	}
	var years []string
	for _, period := range b.Periods {
		years = append(years, fmt.Sprintf("%d", period.StartDate.Year))
	}
	// Get ETF Symbols
	etfscraper, isETFScraper := msg.MustGet(dependency.GetETFScraper).(dependency.ETFScraper)
	if !isETFScraper {
		return errors.New("error casting ETFScraper")
	}
	periodsAndAssets, err := etfscraper.GetAssets(command.ETFName, years...)
	if err != nil {
		return fmt.Errorf("error getting assets: %w", err)
	}
	// Fetch Symbols from Finance
	marketdataClient, isMarketdataClient := msg.MustGet(dependency.GetMarketdataClient).(financeServicePB.MarketdataClient)
	if !isMarketdataClient {
		return errors.New("error casting marketdata client")
	}
	for period, assets := range periodsAndAssets {
		start, err := time.Parse("2006-01-02", fmt.Sprintf("%s-01-01", period))
		if err != nil {
			return fmt.Errorf("error parsing start date: %w", err)
		}
		end, err := time.Parse("2006-01-02", fmt.Sprintf("%s-12-31", period))
		if err != nil {
			return fmt.Errorf("error parsing end date: %w", err)
		}
		backtet_period := backtest.Backtest_Period{
			StartDate: pb.GoTimeToDate(start),
			EndDate:   pb.GoTimeToDate(end),
		}
		for _, asset := range assets {
			rsp, err := marketdataClient.GetAsset(ctx, &financeServicePB.GetAssetRequest{
				Identifier: &financeServicePB.GetAssetRequest_Symbol{
					Symbol: asset.Symbol,
				},
			})
			if err != nil {
				return fmt.Errorf("error getting asset: %w", err)
			}
			backtet_period.Assets = append(backtet_period.Assets, &backtest.Backtest_Period_Asset{
				Symbol:  rsp.Asset.Symbol,
				Enabled: true,
			})
		}
		for _, p := range b.Periods {
			if p.StartDate.Year == backtet_period.StartDate.Year {
				p.Assets = append(p.Assets, backtet_period.Assets...)
				break
			}
		}
	}

	for _, period := range b.Periods {
		err = backtests.UpdatePeriod(ctx, command.BacktestName, period)
		if err != nil {
			return fmt.Errorf("error updating period assets: %w", err)
		}
	}
	return nil
}

func Ingest(ctx context.Context, msg stream.Message) error {
	command := ss.IngestCommand{}

	err := msg.ParsePayload(&command)
	if err != nil {
		return fmt.Errorf("error unmarshalling Ingest payload: %w", err)
	}

	store, isStorage := msg.MustGet(stream.StorageDep).(storage.Storage)
	if !isStorage {
		return errors.New("error casting storage")
	}

	metadata := map[string]string{
		"Start_date": command.Start,
		"End_date":   command.End,
		"Symbols":    strings.Join(command.Symbols, ","),
	}
	name := fmt.Sprintf("%s-%s", command.Start, command.End)

	object, err := store.CreateObject(ctx, storage.IngestionsBucket, name, storage.WithMetadata(metadata))
	if err != nil {
		return fmt.Errorf("error creating ingestion: %w", err)
	}

	ze, err := msg.Call(ctx, dependency.GetEngineKey)
	if err != nil {
		return fmt.Errorf("error getting zipline engine: %w", err)
	}

	engine, isEngine := ze.(engine.Engine)
	if !isEngine {
		return errors.New("error casting zipline engine")
	}

	ingestion := servicePb.Ingestion{
		StartDate: pb.DateStringToDate(command.Start),
		EndDate:   pb.DateStringToDate(command.End),
		Symbols:   command.Symbols,
	}

	err = engine.Ingest(ctx, &ingestion, object)
	if err != nil {
		return fmt.Errorf("error ingesting data: %w", err)
	}
	return nil
}
