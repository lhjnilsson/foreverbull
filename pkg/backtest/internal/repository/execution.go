package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	internal_pb "github.com/lhjnilsson/foreverbull/internal/pb"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/pb"
)

const ExecutionTable = `CREATE TABLE IF NOT EXISTS execution (
id text PRIMARY KEY DEFAULT uuid_generate_v4 (),
session text REFERENCES session(id) ON DELETE CASCADE,
status int NOT NULL DEFAULT 0,
error TEXT,
start_date date NOT NULL,
end_date date,
benchmark text,
symbols text[]);

CREATE TABLE IF NOT EXISTS execution_status (
	id text REFERENCES execution(id) ON DELETE CASCADE,
	status int NOT NULL,
	error text,
	occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);


CREATE OR REPLACE FUNCTION notify_execution_status() RETURNS TRIGGER AS $$
BEGIN
	-- Only update execution_status if the status column is updated
	IF (TG_OP = 'UPDATE' AND OLD.status <> NEW.status) OR TG_OP = 'INSERT' THEN
		INSERT INTO execution_status (id, status, error)
		VALUES (NEW.id, NEW.status, NEW.error);
		PERFORM pg_notify('execution_status', NEW.id);
	END IF;
	RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DO
$$BEGIN
	CREATE TRIGGER execution_status_trigger AFTER INSERT OR UPDATE ON execution
	FOR EACH ROW EXECUTE PROCEDURE notify_execution_status();
EXCEPTION
	WHEN duplicate_object THEN
		NULL;
END$$;

CREATE TABLE IF NOT EXISTS backtest_period (
	id serial primary key,
	backtest_execution text not null,

	date date not null,
	pnl numeric  not null,
	returns numeric  not null,
	portfolio_value numeric  not null,

	longs_count integer,
	shorts_count integer,
	long_value numeric  not null,
	short_value numeric  not null,
	starting_exposure numeric  not null,
	ending_exposure numeric  not null,
	long_exposure numeric  not null,
	short_exposure numeric  not null,

	capital_used numeric  not null,
	gross_leverage numeric  not null,
	net_leverage numeric  not null,

	starting_value numeric  not null,
	ending_value numeric  not null,
	starting_cash numeric  not null,
	ending_cash numeric  not null,

	max_drawdown numeric  not null,
	max_leverage numeric  not null,
	excess_returns numeric  not null,
	treasury_period_return numeric  not null,
	algorithm_period_return numeric  not null,

	algo_volatility numeric,
	sharpe numeric,
	sortino numeric,

	benchmark_period_return numeric,
	benchmark_volatility numeric,
	alpha numeric,
	beta numeric
);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'unique_backtest_period'
        AND conrelid = 'backtest_period'::regclass
    ) THEN
        ALTER TABLE backtest_period ADD CONSTRAINT unique_backtest_period UNIQUE(backtest_execution, date);
    END IF;
END $$;
`

type Execution struct {
	Conn postgres.Query
}

func (db *Execution) Create(ctx context.Context, session string, start, end *internal_pb.Date,
	symbols []string, benchmark *string,
) (*pb.Execution, error) {
	var executionID string

	var endDate *string

	if end != nil {
		ed := internal_pb.DateToDateString(end)
		endDate = &ed
	}

	err := db.Conn.QueryRow(ctx,
		`INSERT INTO execution (session, start_date, end_date, benchmark, symbols)
		VALUES($1,$2,$3,$4,$5) RETURNING id`, session, internal_pb.DateToDateString(start),
		endDate, benchmark, symbols).
		Scan(&executionID)
	if err != nil {
		return nil, fmt.Errorf("failed to create execution: %w", err)
	}

	return db.Get(ctx, executionID)
}

func (db *Execution) StorePeriods(ctx context.Context, execution string, periods []*pb.Period) error {
	for _, period := range periods {
		_, err := db.Conn.Exec(ctx,
			`INSERT INTO backtest_period(
				backtest_execution, date, pnl, returns, portfolio_value,
				longs_count, shorts_count, long_value, short_value, starting_exposure, ending_exposure, long_exposure, short_exposure,
				capital_used, gross_leverage, net_leverage,
				starting_value, ending_value, starting_cash, ending_cash,
				max_drawdown, max_leverage, excess_returns, treasury_period_return, algorithm_period_return,
				algo_volatility, sharpe, sortino,
				benchmark_period_return, benchmark_volatility, alpha, beta)
			VALUES($1, $2, $3, $4, $5,
				$6, $7, $8, $9, $10, $11, $12, $13,
				$14, $15, $16,
				$17, $18, $19, $20,
				$21, $22, $23, $24, $25,
				$26, $27, $28,
				$29, $30, $31, $32)`,
			execution, internal_pb.DateToDateString(period.Date), period.PNL, period.Returns, period.PortfolioValue,
			period.LongsCount, period.ShortsCount, period.LongValue, period.ShortValue, period.StartingExposure, period.EndingExposure, period.LongExposure, period.ShortExposure,
			period.CapitalUsed, period.GrossLeverage, period.NetLeverage,
			period.StartingValue, period.EndingValue, period.StartingCash, period.EndingCash,
			period.MaxDrawdown, period.MaxLeverage, period.ExcessReturn, period.TreasuryPeriodReturn, period.AlgorithmPeriodReturn,
			period.AlgoVolatility, period.Sharpe, period.Sortino,
			period.BenchmarkPeriodReturn, period.BenchmarkVolatility, period.Alpha, period.Beta,
		)
		if err != nil {
			return fmt.Errorf("error creating period result: %w", err)
		}
	}

	return nil
}

func (db *Execution) Get(ctx context.Context, executionId string) (*pb.Execution, error) {
	execution := pb.Execution{}

	rows, err := db.Conn.Query(ctx,
		`SELECT execution.id, session, start_date, end_date, benchmark, symbols,
		es.status, es.error, es.occurred_at
		FROM execution
		INNER JOIN (
			SELECT id, status, error, occurred_at FROM execution_status ORDER BY occurred_at DESC
		) AS es ON execution.id=es.id
		WHERE execution.id=$1`, executionId)
	if err != nil {
		return nil, fmt.Errorf("failed to get execution: %w", err)
	}

	defer rows.Close()

	for rows.Next() {
		status := pb.Execution_Status{}
		start := time.Time{}
		end := pgtype.Date{}
		occurred_at := time.Time{}

		err = rows.Scan(&execution.Id, &execution.Session, &start, &end, &execution.Benchmark,
			&execution.Symbols, &status.Status, &status.Error, &occurred_at)
		if err != nil {
			return nil, fmt.Errorf("failed to scan execution: %w", err)
		}

		execution.StartDate = internal_pb.GoTimeToDate(start)
		if end.Valid {
			execution.EndDate = internal_pb.GoTimeToDate(end.Time)
		}

		status.OccurredAt = internal_pb.TimeToProtoTimestamp(occurred_at)
		execution.Statuses = append(execution.Statuses, &status)
	}

	if execution.Id == "" {
		return nil, errors.New("execution not found")
	}

	return &execution, nil
}

func (db *Execution) GetPeriods(ctx context.Context, executionId string) ([]*pb.Period, error) {
	rows, err := db.Conn.Query(ctx,
		`SELECT date, pnl, returns, portfolio_value, longs_count, shorts_count,
		long_value, short_value, starting_exposure, ending_exposure, long_exposure, short_exposure, capital_used, gross_leverage,
		net_leverage, starting_value, ending_value, starting_cash, ending_cash, max_drawdown,
		max_leverage, excess_returns, treasury_period_return, algorithm_period_return,
		algo_volatility, sharpe, sortino, benchmark_period_return, benchmark_volatility,
		alpha, beta FROM backtest_period WHERE backtest_execution=$1 ORDER BY DATE ASC`, executionId)
	if err != nil {
		return nil, fmt.Errorf("failed to get periods: %w", err)
	}

	periods := make([]*pb.Period, 0)
	defer rows.Close()
	for rows.Next() {
		period := pb.Period{}
		periodDate := pgtype.Date{}
		pnl := sql.NullFloat64{}
		returns := sql.NullFloat64{}
		portfolioValue := sql.NullFloat64{}
		longsCount := sql.NullInt32{}
		shortsCount := sql.NullInt32{}
		longValue := sql.NullFloat64{}
		shortValue := sql.NullFloat64{}
		startingExposure := sql.NullFloat64{}
		endingExposure := sql.NullFloat64{}
		longExposure := sql.NullFloat64{}
		shortExposure := sql.NullFloat64{}
		capitalUsed := sql.NullFloat64{}
		grossLeverage := sql.NullFloat64{}
		netLeverage := sql.NullFloat64{}
		startingValue := sql.NullFloat64{}
		endingValue := sql.NullFloat64{}
		startingCash := sql.NullFloat64{}
		endingCash := sql.NullFloat64{}
		maxDrawdown := sql.NullFloat64{}
		maxLeverage := sql.NullFloat64{}
		excessReturn := sql.NullFloat64{}
		treasuryPeriodReturn := sql.NullFloat64{}
		algorithmPeriodReturn := sql.NullFloat64{}
		algoVolatility := sql.NullFloat64{}
		sharpe := sql.NullFloat64{}
		sortino := sql.NullFloat64{}
		benchmarkPeriodReturn := sql.NullFloat64{}
		benchmarkVolatility := sql.NullFloat64{}
		alpha := sql.NullFloat64{}
		beta := sql.NullFloat64{}

		err = rows.Scan(&periodDate, &pnl, &returns, &portfolioValue, &longsCount, &shortsCount,
			&longValue, &shortValue, &startingExposure, &endingExposure, &longExposure, &shortExposure, &capitalUsed, &grossLeverage,
			&netLeverage, &startingValue, &endingValue, &startingCash, &endingCash, &maxDrawdown,
			&maxLeverage, &excessReturn, &treasuryPeriodReturn, &algorithmPeriodReturn,
			&algoVolatility, &sharpe, &sortino, &benchmarkPeriodReturn, &benchmarkVolatility,
			&alpha, &beta)
		if err != nil {
			return nil, fmt.Errorf("failed to scan period: %w", err)
		}
		period.Date = internal_pb.GoTimeToDate(periodDate.Time)
		if pnl.Valid {
			period.PNL = pnl.Float64
		}
		if returns.Valid {
			period.Returns = returns.Float64
		}
		if portfolioValue.Valid {
			period.PortfolioValue = portfolioValue.Float64
		}
		if longsCount.Valid {
			period.LongsCount = longsCount.Int32
		}
		if shortsCount.Valid {
			period.ShortsCount = shortsCount.Int32
		}
		if longValue.Valid {
			period.LongValue = longValue.Float64
		}
		if shortValue.Valid {
			period.ShortValue = shortValue.Float64
		}
		if startingExposure.Valid {
			period.StartingExposure = startingExposure.Float64
		}
		if endingExposure.Valid {
			period.EndingExposure = endingExposure.Float64
		}
		if longExposure.Valid {
			period.LongExposure = longExposure.Float64
		}
		if shortExposure.Valid {
			period.ShortExposure = shortExposure.Float64
		}
		if capitalUsed.Valid {
			period.CapitalUsed = capitalUsed.Float64
		}
		if grossLeverage.Valid {
			period.GrossLeverage = grossLeverage.Float64
		}
		if netLeverage.Valid {
			period.NetLeverage = netLeverage.Float64
		}
		if startingValue.Valid {
			period.StartingValue = startingValue.Float64
		}
		if endingValue.Valid {
			period.EndingValue = endingValue.Float64
		}
		if startingCash.Valid {
			period.StartingCash = startingCash.Float64
		}
		if endingCash.Valid {
			period.EndingCash = endingCash.Float64
		}
		if maxDrawdown.Valid {
			period.MaxDrawdown = maxDrawdown.Float64
		}
		if maxLeverage.Valid {
			period.MaxLeverage = maxLeverage.Float64
		}
		if excessReturn.Valid {
			period.ExcessReturn = excessReturn.Float64
		}
		if treasuryPeriodReturn.Valid {
			period.TreasuryPeriodReturn = treasuryPeriodReturn.Float64
		}
		if algorithmPeriodReturn.Valid {
			period.AlgorithmPeriodReturn = algorithmPeriodReturn.Float64
		}
		if algoVolatility.Valid {
			period.AlgoVolatility = &algoVolatility.Float64
		}
		if sharpe.Valid {
			period.Sharpe = &sharpe.Float64
		}
		if sortino.Valid {
			period.Sortino = &sortino.Float64
		}
		if benchmarkPeriodReturn.Valid {
			period.BenchmarkPeriodReturn = &benchmarkPeriodReturn.Float64
		}
		if benchmarkVolatility.Valid {
			period.BenchmarkVolatility = &benchmarkVolatility.Float64
		}
		if alpha.Valid {
			period.Alpha = &alpha.Float64
		}
		if beta.Valid {
			period.Beta = &beta.Float64
		}

		periods = append(periods, &period)
	}

	return periods, nil
}

func (db *Execution) UpdateSimulationDetails(ctx context.Context, e *pb.Execution) error {
	_, err := db.Conn.Exec(ctx,
		`UPDATE execution SET start_date=$1, end_date=$2, benchmark=$3,
		symbols=$4 WHERE id=$5`, internal_pb.DateToDateString(e.StartDate),
		internal_pb.DateToDateString(e.EndDate), e.Benchmark, e.Symbols, e.Id)
	if err != nil {
		return fmt.Errorf("failed to update execution: %w", err)
	}

	return nil
}

func (db *Execution) UpdateStatus(ctx context.Context, executionId string, status pb.Execution_Status_Status, err error) error {
	if err != nil {
		_, err = db.Conn.Exec(ctx, "UPDATE execution SET status=$2, error=$3 WHERE id=$1", executionId, status, err.Error())
	} else {
		_, err = db.Conn.Exec(ctx, "UPDATE execution SET status=$2 WHERE id=$1", executionId, status)
	}

	if err != nil {
		return fmt.Errorf("failed to update execution status: %w", err)
	}

	return nil
}

func (db *Execution) parseRows(rows pgx.Rows) ([]*pb.Execution, error) {
	executions := make([]*pb.Execution, 0)

	var err error

	var inReturnSlice bool

	for rows.Next() {
		status := pb.Execution_Status{}
		execution := pb.Execution{}
		result := pb.Period{}
		resultDate := pgtype.Date{}
		start := time.Time{}
		end := pgtype.Date{}
		occurredAt := time.Time{}

		pnl := sql.NullFloat64{}
		returns := sql.NullFloat64{}
		portfolioValue := sql.NullFloat64{}
		longsCount := sql.NullInt32{}
		shortsCount := sql.NullInt32{}
		longValue := sql.NullFloat64{}
		shortValue := sql.NullFloat64{}
		startingExposure := sql.NullFloat64{}
		endingExposure := sql.NullFloat64{}
		longExposure := sql.NullFloat64{}
		shortExposure := sql.NullFloat64{}
		capitalUsed := sql.NullFloat64{}
		grossLeverage := sql.NullFloat64{}
		netLeverage := sql.NullFloat64{}
		startingValue := sql.NullFloat64{}
		endingValue := sql.NullFloat64{}
		startingCash := sql.NullFloat64{}
		endingCash := sql.NullFloat64{}
		maxDrawdown := sql.NullFloat64{}
		maxLeverage := sql.NullFloat64{}
		excessReturn := sql.NullFloat64{}
		treasuryPeriodReturn := sql.NullFloat64{}
		algorithmPeriodReturn := sql.NullFloat64{}
		algoVolatility := sql.NullFloat64{}
		sharpe := sql.NullFloat64{}
		sortino := sql.NullFloat64{}
		benchmarkPeriodReturn := sql.NullFloat64{}
		benchmarkVolatility := sql.NullFloat64{}
		alpha := sql.NullFloat64{}
		beta := sql.NullFloat64{}

		err = rows.Scan(&execution.Id, &execution.Session, &execution.Backtest, &start, &end, &execution.Benchmark,
			&execution.Symbols, &status.Status, &status.Error, &occurredAt,
			&resultDate, &pnl, &returns, &portfolioValue, &longsCount, &shortsCount,
			&longValue, &shortValue, &startingExposure, &endingExposure, &longExposure,
			&shortExposure, &capitalUsed, &grossLeverage, &netLeverage, &startingValue,
			&endingValue, &startingCash, &endingCash, &maxDrawdown, &maxLeverage,
			&excessReturn, &treasuryPeriodReturn, &algorithmPeriodReturn, &algoVolatility,
			&sharpe, &sortino, &benchmarkPeriodReturn, &benchmarkVolatility, &alpha, &beta)
		if err != nil {
			return nil, fmt.Errorf("failed to scan execution: %w", err)
		}

		execution.StartDate = internal_pb.GoTimeToDate(start)
		if end.Valid {
			execution.EndDate = internal_pb.GoTimeToDate(end.Time)
		}

		status.OccurredAt = internal_pb.TimeToProtoTimestamp(occurredAt)

		inReturnSlice = false

		for i := range executions {
			if executions[i].Id == execution.Id {
				executions[i].Statuses = append(executions[i].Statuses, &status)
				inReturnSlice = true
				break
			}
		}

		if !inReturnSlice {
			execution.Statuses = append(execution.Statuses, &status)
			executions = append(executions, &execution)
		}

		result.Date = internal_pb.GoTimeToDate(resultDate.Time)
		if pnl.Valid {
			result.PNL = pnl.Float64
		}
		if returns.Valid {
			result.Returns = returns.Float64
		}
		if portfolioValue.Valid {
			result.PortfolioValue = portfolioValue.Float64
		}
		if longsCount.Valid {
			result.LongsCount = longsCount.Int32
		}
		if shortsCount.Valid {
			result.ShortsCount = shortsCount.Int32
		}
		if longValue.Valid {
			result.LongValue = longValue.Float64
		}
		if shortValue.Valid {
			result.ShortValue = shortValue.Float64
		}
		if startingExposure.Valid {
			result.StartingExposure = startingExposure.Float64
		}
		if endingExposure.Valid {
			result.EndingExposure = endingExposure.Float64
		}
		if longExposure.Valid {
			result.LongExposure = longExposure.Float64
		}
		if shortExposure.Valid {
			result.ShortExposure = shortExposure.Float64
		}
		if capitalUsed.Valid {
			result.CapitalUsed = capitalUsed.Float64
		}
		if grossLeverage.Valid {
			result.GrossLeverage = grossLeverage.Float64
		}
		if netLeverage.Valid {
			result.NetLeverage = netLeverage.Float64
		}
		if startingValue.Valid {
			result.StartingValue = startingValue.Float64
		}
		if endingValue.Valid {
			result.EndingValue = endingValue.Float64
		}
		if startingCash.Valid {
			result.StartingCash = startingCash.Float64
		}
		if endingCash.Valid {
			result.EndingCash = endingCash.Float64
		}
		if maxDrawdown.Valid {
			result.MaxDrawdown = maxDrawdown.Float64
		}
		if maxLeverage.Valid {
			result.MaxLeverage = maxLeverage.Float64
		}
		if excessReturn.Valid {
			result.ExcessReturn = excessReturn.Float64
		}
		if treasuryPeriodReturn.Valid {
			result.TreasuryPeriodReturn = treasuryPeriodReturn.Float64
		}
		if algorithmPeriodReturn.Valid {
			result.AlgorithmPeriodReturn = algorithmPeriodReturn.Float64
		}
		if algoVolatility.Valid {
			result.AlgoVolatility = &algoVolatility.Float64
		}
		if sharpe.Valid {
			result.Sharpe = &sharpe.Float64
		}
		if sortino.Valid {
			result.Sortino = &sortino.Float64
		}
		if benchmarkPeriodReturn.Valid {
			result.BenchmarkPeriodReturn = &benchmarkPeriodReturn.Float64
		}
		if benchmarkVolatility.Valid {
			result.BenchmarkVolatility = &benchmarkVolatility.Float64
		}
		if alpha.Valid {
			result.Alpha = &alpha.Float64
		}
		if beta.Valid {
			result.Beta = &beta.Float64
		}
		execution.Result = &result
	}

	return executions, nil
}

func (db *Execution) List(ctx context.Context) ([]*pb.Execution, error) {
	rows, err := db.Conn.Query(ctx,
		`SELECT execution.id, session.id, session.backtest, execution.start_date, execution.end_date, benchmark, symbols,
		es.status, es.error, es.occurred_at,
		ep.date, ep.pnl, ep.returns, ep.portfolio_value, ep.longs_count, ep.shorts_count,
		ep.long_value, ep.short_value, ep.starting_exposure, ep.ending_exposure, ep.long_exposure, ep.short_exposure,
		ep.capital_used, ep.gross_leverage, ep.net_leverage,
		ep.starting_value, ep.ending_value, ep.starting_cash, ep.ending_cash,
		ep.max_drawdown, ep.max_leverage, ep.excess_returns, ep.treasury_period_return, ep.algorithm_period_return,
		ep.algo_volatility, ep.sharpe, ep.sortino,
		ep.benchmark_period_return, ep.benchmark_volatility, ep.alpha, ep.beta
		FROM execution
		INNER JOIN session ON execution.session=session.id
		INNER JOIN (
			SELECT id, status, error, occurred_at FROM execution_status ORDER BY occurred_at DESC
		) AS es ON execution.id=es.id
		LEFT JOIN (
			SELECT backtest_execution, date, pnl, returns, portfolio_value, longs_count, shorts_count,
			long_value, short_value, starting_exposure, ending_exposure, long_exposure, short_exposure, capital_used, gross_leverage,
			net_leverage, starting_value, ending_value, starting_cash, ending_cash, max_drawdown,
			max_leverage, excess_returns, treasury_period_return, algorithm_period_return,
			algo_volatility, sharpe, sortino, benchmark_period_return, benchmark_volatility,
			alpha, beta FROM backtest_period ORDER BY date DESC LIMIT 1
		) as ep ON execution.id=ep.backtest_execution
		ORDER BY es.occurred_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("failed to list executions: %w", err)
	}

	defer rows.Close()

	return db.parseRows(rows)
}

func (db *Execution) ListBySession(ctx context.Context, session string) ([]*pb.Execution, error) {
	rows, err := db.Conn.Query(ctx,
		`SELECT execution.id, session.id, session.backtest, execution.start_date, execution.end_date, benchmark, symbols,
		es.status, es.error, es.occurred_at,
		ep.date, ep.pnl, ep.returns, ep.portfolio_value, ep.longs_count, ep.shorts_count,
		ep.long_value, ep.short_value, ep.starting_exposure, ep.ending_exposure, ep.long_exposure, ep.short_exposure,
		ep.capital_used, ep.gross_leverage, ep.net_leverage,
		ep.starting_value, ep.ending_value, ep.starting_cash, ep.ending_cash,
		ep.max_drawdown, ep.max_leverage, ep.excess_returns, ep.treasury_period_return, ep.algorithm_period_return,
		ep.algo_volatility, ep.sharpe, ep.sortino,
		ep.benchmark_period_return, ep.benchmark_volatility, ep.alpha, ep.beta
		FROM execution
		INNER JOIN session ON execution.session=session.id
		INNER JOIN (
			SELECT id, status, error, occurred_at FROM execution_status ORDER BY occurred_at DESC
		) AS es ON execution.id=es.id
		LEFT JOIN (
			SELECT backtest_execution, date, pnl, returns, portfolio_value, longs_count, shorts_count,
			long_value, short_value, starting_exposure, ending_exposure, long_exposure, short_exposure, capital_used, gross_leverage,
			net_leverage, starting_value, ending_value, starting_cash, ending_cash, max_drawdown,
			max_leverage, excess_returns, treasury_period_return, algorithm_period_return,
			algo_volatility, sharpe, sortino, benchmark_period_return, benchmark_volatility,
			alpha, beta FROM backtest_period ORDER BY date DESC LIMIT 1
		) as ep ON execution.id=ep.backtest_execution
		WHERE session=$1
		ORDER BY es.occurred_at ASC`, session)
	if err != nil {
		return nil, fmt.Errorf("failed to list executions: %w", err)
	}

	defer rows.Close()

	return db.parseRows(rows)
}

func (db *Execution) ListByBacktest(ctx context.Context, backtest string) ([]*pb.Execution, error) {
	rows, err := db.Conn.Query(ctx,
		`SELECT execution.id, session.id, session.backtest, execution.start_date, execution.end_date,
		execution.benchmark, execution.symbols,
		es.status, es.error, es.occurred_at,
		ep.date, ep.pnl, ep.returns, ep.portfolio_value, ep.longs_count, ep.shorts_count,
		ep.long_value, ep.short_value, ep.starting_exposure, ep.ending_exposure, ep.long_exposure, ep.short_exposure,
		ep.capital_used, ep.gross_leverage, ep.net_leverage,
		ep.starting_value, ep.ending_value, ep.starting_cash, ep.ending_cash,
		ep.max_drawdown, ep.max_leverage, ep.excess_returns, ep.treasury_period_return, ep.algorithm_period_return,
		ep.algo_volatility, ep.sharpe, ep.sortino,
		ep.benchmark_period_return, ep.benchmark_volatility, ep.alpha, ep.beta
		FROM execution
		INNER JOIN (
			SELECT id, status, error, occurred_at FROM execution_status ORDER BY occurred_at DESC
		) AS es ON execution.id=es.id
		LEFT JOIN (
			SELECT backtest_execution, date, pnl, returns, portfolio_value, longs_count, shorts_count,
			long_value, short_value, starting_exposure, ending_exposure, long_exposure, short_exposure, capital_used, gross_leverage,
			net_leverage, starting_value, ending_value, starting_cash, ending_cash, max_drawdown,
			max_leverage, excess_returns, treasury_period_return, algorithm_period_return,
			algo_volatility, sharpe, sortino, benchmark_period_return, benchmark_volatility,
			alpha, beta FROM backtest_period ORDER BY date DESC LIMIT 1
		) as ep ON execution.id=ep.backtest_execution
		INNER JOIN session ON execution.session=session.id
		INNER JOIN backtest ON session.backtest=backtest.name
		WHERE backtest.name=$1
		ORDER BY es.occurred_at ASC`, backtest)
	if err != nil {
		return nil, fmt.Errorf("failed to list executions: %w", err)
	}

	defer rows.Close()

	return db.parseRows(rows)
}
