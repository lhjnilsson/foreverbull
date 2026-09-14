package repository_test

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/repository"
	"google.golang.org/protobuf/types/known/structpb"
)

func insertService(ctx context.Context, conn *pgxpool.Pool, image string) error {
	_, err := conn.Exec(ctx, `INSERT INTO service (image) VALUES ($1)`, image)
	return err
}

func insertExecution(ctx context.Context, conn *pgxpool.Pool, backtestName string) (string, error) {
	_, err := conn.Exec(ctx,
		`INSERT INTO backtest (name, benchmark) VALUES ($1, $2)`,
		backtestName, "SPY")
	if err != nil {
		return "", err
	}

	var sessionID string
	err = conn.QueryRow(ctx,
		`INSERT INTO session (backtest) VALUES ($1) RETURNING id`,
		backtestName,
	).Scan(&sessionID)
	if err != nil {
		return "", err
	}

	var executionID string
	err = conn.QueryRow(ctx,
		`INSERT INTO backtest_execution (session, start_date, end_date)
		VALUES ($1, $2, $3) RETURNING id`,
		sessionID, "2020-01-01", "2020-12-31",
	).Scan(&executionID)
	if err != nil {
		return "", err
	}

	return executionID, nil
}

func createStrategyAndVersion(
	ctx context.Context,
	conn *pgxpool.Pool,
	name, image string,
	parameters *structpb.Struct,
) (*pb.Strategy, *pb.Version, error) {
	if err := insertService(ctx, conn, image); err != nil {
		return nil, nil, err
	}

	strategies := repository.Strategy{Conn: conn}
	strategy, err := strategies.Create(ctx, name, nil)
	if err != nil {
		return nil, nil, err
	}

	versions := repository.Version{Conn: conn}
	version, err := versions.Create(ctx, name, image, parameters)
	if err != nil {
		return nil, nil, err
	}

	return strategy, version, nil
}
