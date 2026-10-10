package finance

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/api"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/stream/command"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/stream/dependency"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/marketdata"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/trading"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"go.uber.org/fx"
	"google.golang.org/grpc"
)

const StreamName = "finance"

type Subscriber stream.Subscriber

var Module = fx.Options( //nolint: gochecknoglobals
	fx.Provide(
		func() (supplier.Marketdata, error) {
			switch environment.GetMarketDataProvider() {
			case "alpaca":
				marketData, err := marketdata.NewAlpacaClient()
				if err != nil {
					return nil, fmt.Errorf("failed to create Alpaca client: %w", err)
				}
				return marketData, nil
			case "yahoo":
				marketData, err := marketdata.NewYahooClient()
				if err != nil {
					return nil, fmt.Errorf("failed to create Yahoo client: %w", err)
				}
				return marketData, nil
			case "stooq":
				marketData, err := marketdata.NewStooqYahoo(marketdata.NewYahooClient)
				if err != nil {
					return nil, err
				}
				return marketData, nil
			default:
				return nil, fmt.Errorf("unknown market data provider: %s", environment.GetMarketDataProvider())
			}
		},
		func(conn *pgxpool.Pool) (supplier.Trading, error) {
			if !environment.GetTradingEnabled() {
				return nil, nil
			}

			client, err := trading.New(trading.Config{
				Provider:        environment.GetTradingProvider(),
				Currency:        environment.GetTradingCurrency(),
				AllowLiveURL:    environment.GetTradingAllowLiveURL(),
				ListingsFile:    environment.GetTradingListingsFile(),
				Directory:       newSymbolDirectory(conn),
				AlpacaBaseURL:   environment.GetAlpacaBaseURL(),
				AlpacaAPIKey:    environment.GetAlpacaAPIKey(),
				AlpacaAPISecret: environment.GetAlpacaAPISecret(),
				SaxoBaseURL:     environment.GetSaxoBaseURL(),
				SaxoAccessToken: environment.GetSaxoAccessToken(),
				SaxoAccountKey:  environment.GetSaxoAccountKey(),
				IBKRBaseURL:     environment.GetIBKRBaseURL(),
				IBKRAccountID:   environment.GetIBKRAccountID(),
				IBKRAccountMode: environment.GetIBKRAccountMode(),
				IBKRAccessToken: environment.GetIBKRAccessToken(),
				IGBaseURL:       environment.GetIGBaseURL(),
				IGAPIKey:        environment.GetIGAPIKey(),
				IGIdentifier:    environment.GetIGIdentifier(),
				IGPassword:      environment.GetIGPassword(),
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create trading client: %w", err)
			}
			return client, nil
		},
		func(conn *pgxpool.Pool, marketData supplier.Marketdata, trading supplier.Trading, eventStream stream.Stream) (Subscriber, error) {
			dc := stream.NewDependencyContainer()
			dc.AddSingleton(stream.DBDep, conn)
			dc.AddSingleton(dependency.MarketDataDep, marketData)
			dc.AddSingleton(dependency.TradingDep, trading)
			sub, err := eventStream.NewSubscriber(StreamName, dc)
			if err != nil {
				return nil, fmt.Errorf("failed to create subscriber: %w", err)
			}
			return sub, nil
		},
	),
	fx.Invoke(
		func(s *grpc.Server, pgx *pgxpool.Pool, eventStream stream.Stream, md supplier.Marketdata, t supplier.Trading) error {
			marketdataServer := api.NewMarketdataServer(pgx, md)
			servicePb.RegisterMarketdataServer(s, marketdataServer)
			tradingServer := api.NewTradingServer(pgx, eventStream, t)
			servicePb.RegisterTradingServicerServer(s, tradingServer)
			return nil
		},
		func(conn *pgxpool.Pool) error {
			return repository.CreateTables(context.Background(), conn)
		},
		func(lc fx.Lifecycle, sub Subscriber) error {
			lc.Append(fx.Hook{
				OnStart: func(_ context.Context) error {
					err := sub.Subscribe("marketdata", "ingest", command.Ingest)
					if err != nil {
						return fmt.Errorf("failed to subscribe to ingest command: %w", err)
					}
					err = sub.Subscribe("trading", "run", command.SessionRun)
					if err != nil {
						return fmt.Errorf("failed to subscribe to run command: %w", err)
					}
					return nil
				},
				OnStop: func(_ context.Context) error {
					return sub.Unsubscribe()
				},
			})
			return nil
		},
	),
)

// symbolDirectory reads instrument and asset_symbol. Ticker is not identity.
// Zero or several ISINs are returned to the wrapper; they are not dropped.
type symbolDirectory struct {
	conn *pgxpool.Pool
}

func newSymbolDirectory(conn *pgxpool.Pool) trading.Directory {
	return symbolDirectory{conn: conn}
}

func (d symbolDirectory) Resolve(ctx context.Context, symbol string, at time.Time) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	symbol = strings.TrimSpace(symbol)
	if symbol == "" || d.conn == nil {
		return nil, nil
	}

	year, month, day := at.Date()
	at = time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	rows, err := d.conn.Query(ctx, `
		SELECT DISTINCT i.isin
		  FROM asset_symbol s
		  JOIN instrument i ON i.isin = s.isin
		 WHERE s.symbol = $1
		   AND s.from_date <= $2::date
		   AND (s.to_date IS NULL OR $2::date < s.to_date)
		 ORDER BY i.isin`,
		symbol, at.Format("2006-01-02"))
	if err != nil {
		return nil, fmt.Errorf("resolve symbol: %w", err)
	}
	defer rows.Close()

	var out []string
	seen := make(map[string]struct{})
	for rows.Next() {
		var isin string
		if err := rows.Scan(&isin); err != nil {
			return nil, fmt.Errorf("resolve symbol: %w", err)
		}
		isin = strings.ToUpper(strings.TrimSpace(isin))
		if isin == "" {
			continue
		}
		if _, ok := seen[isin]; ok {
			continue
		}
		seen[isin] = struct{}{}
		out = append(out, isin)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("resolve symbol: %w", err)
	}
	return out, nil
}
