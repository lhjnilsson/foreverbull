package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/pkg/finance/pb"
)

const AssetTable = `CREATE TABLE IF NOT EXISTS asset (
symbol text primary key,
name text);
`

type Asset struct {
	Conn postgres.Query
}

func (db *Asset) List(ctx context.Context) ([]*pb.Asset, error) {
	rows, err := db.Conn.Query(
		ctx,
		`SELECT symbol, name FROM asset`,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list assets: %w", err)
	}

	assets := make([]*pb.Asset, 0)

	for rows.Next() {
		a := pb.Asset{}

		err := rows.Scan(&a.Symbol, &a.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to scan asset: %w", err)
		}

		assets = append(assets, &a)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("failed to list assets: %w", err)
	}

	return assets, nil
}

func (db *Asset) ListBySymbols(ctx context.Context, symbols []string) ([]*pb.Asset, error) {
	rows, err := db.Conn.Query(
		ctx,
		"SELECT symbol, name  FROM asset WHERE symbol = ANY($1)",
		symbols,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list assets: %w", err)
	}

	assets := make([]*pb.Asset, 0)

	for rows.Next() {
		a := pb.Asset{}

		err := rows.Scan(&a.Symbol, &a.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to scan asset: %w", err)
		}

		assets = append(assets, &a)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("failed to list assets: %w", err)
	}

	if len(assets) != len(symbols) {
		return nil, errors.New("not all symbols found")
	}

	return assets, nil
}

func (db *Asset) Store(ctx context.Context, symbol, name string) error {
	_, err := db.Conn.Exec(ctx,
		`INSERT INTO asset(symbol, name) values($1, $2)
		ON CONFLICT DO NOTHING`, symbol, name)
	if err != nil {
		return fmt.Errorf("failed to store asset: %w", err)
	}

	return nil
}

func (db *Asset) Get(ctx context.Context, symbol string) (*pb.Asset, error) {
	a := pb.Asset{Symbol: symbol}

	err := db.Conn.QueryRow(ctx,
		"SELECT name FROM asset WHERE symbol=$1", symbol).Scan(
		&a.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get asset: %w", err)
	}

	return &a, nil
}

func (db *Asset) Delete(ctx context.Context, symbol string) error {
	_, err := db.Conn.Exec(ctx,
		"DELETE FROM asset WHERE symbol=$1", symbol)
	if err != nil {
		return fmt.Errorf("failed to delete asset: %w", err)
	}

	return nil
}
