package repository

import (
	"context"
	"errors"

	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/pkg/finance/entity"
)

const AssetTable = `CREATE TABLE IF NOT EXISTS asset (
symbol text primary key,
name text,
title text,
asset_type text
);
`

type Asset struct {
	Conn postgres.Query
}

/*
List
List all assets stored
*/
func (db *Asset) List(ctx context.Context) (*[]entity.Asset, error) {
	rows, err := db.Conn.Query(
		ctx,
		`SELECT asset.symbol, name, title, asset_type, min(ohlc.time), max(ohlc.time) FROM asset
		LEFT JOIN ohlc ON ohlc.symbol = asset.symbol GROUP BY asset.symbol`,
	)
	if err != nil {
		return nil, err
	}
	assets := make([]entity.Asset, 0)
	for rows.Next() {
		i := entity.Asset{}
		err := rows.Scan(&i.Symbol, &i.Name, &i.Title, &i.Type, &i.StartOHLC, &i.EndOHLC)
		if err != nil {
			return nil, err
		}
		assets = append(assets, i)
	}
	if rows.Err() != nil {
		return nil, err
	}
	return &assets, nil
}

/*
ListBySymbols
List all assets stored based on list of symbols
*/
func (db *Asset) ListBySymbols(ctx context.Context, symbols []string) (*[]entity.Asset, error) {
	rows, err := db.Conn.Query(
		ctx,
		"SELECT symbol, name, title, asset_type FROM asset WHERE symbol = ANY($1)",
		symbols,
	)
	if err != nil {
		return nil, err
	}
	assets := make([]entity.Asset, 0)
	for rows.Next() {
		i := entity.Asset{}
		err := rows.Scan(&i.Symbol, &i.Name, &i.Title, &i.Type)
		if err != nil {
			return nil, err
		}
		assets = append(assets, i)
	}
	if rows.Err() != nil {
		return nil, err
	}
	if len(assets) != len(symbols) {
		return nil, errors.New("not all symbols found")
	}
	return &assets, nil
}

/*
Create
Create a new asset
*/
func (db *Asset) Store(ctx context.Context, i *entity.Asset) error {
	_, err := db.Conn.Exec(ctx,
		`INSERT INTO asset(symbol, name, title, asset_type) values($1, $2, $3, $4)
		ON CONFLICT DO NOTHING`, i.Symbol, i.Name, i.Title, i.Type)
	return err
}

/*
Get
Get a asset based on asset symbol
*/
func (db *Asset) Get(ctx context.Context, symbol string) (*entity.Asset, error) {
	a := entity.Asset{Symbol: symbol}
	err := db.Conn.QueryRow(ctx,
		"SELECT name, title, asset_type FROM asset WHERE symbol=$1", symbol).Scan(
		&a.Name, &a.Title, &a.Type)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

/*
Delete
Delete asset
*/
func (db *Asset) Delete(ctx context.Context, symbol string) error {
	_, err := db.Conn.Exec(ctx,
		"DELETE FROM asset WHERE symbol=$1", symbol)
	return err
}
