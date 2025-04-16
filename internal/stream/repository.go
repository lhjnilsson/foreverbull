package stream

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/stream/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func CreateTables(ctx context.Context, conn *pgxpool.Pool) error {
	_, err := conn.Exec(ctx, table)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	return nil
}

func RecreateTables(ctx context.Context, conn *pgxpool.Pool) error {
	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS message_states;`); err != nil {
		return fmt.Errorf("failed to drop table: %w", err)
	}

	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS message;`); err != nil {
		return fmt.Errorf("failed to drop table: %w", err)
	}

	_, err := conn.Exec(ctx, table)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	return nil
}

const table = `
CREATE TABLE IF NOT EXISTS message (
	id text PRIMARY KEY,
	module text NOT NULL,
	component text NOT NULL,
	method text NOT NULL,
	payload BYTEA,

	orchestration_name text,
	orchestration_id text,
	orchestration_step text,
	orchestration_step_number integer,
	orchestration_fallback_step boolean
);

CREATE TABLE IF NOT EXISTS message_states (
	id serial PRIMARY KEY,
	message_id text,
	state int NOT NULL,
	error text,
	occurred_at TIMESTAMPTZ NOT NULL
);
`

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateMessage(ctx context.Context, msg *pb.Message) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO message (id, module, component, method, payload,
	orchestration_name, orchestration_id, orchestration_step, orchestration_step_number, orchestration_fallback_step)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`, msg.Id,
		msg.Module, msg.Component, msg.Method, msg.Payload,
		msg.OrchestrationName, msg.OrchestrationId, msg.OrchestrationStep, msg.OrchestrationStepNumber, msg.OrchestrationFallbackStep)
	if err != nil {
		return fmt.Errorf("failed to insert message: %w", err)
	}
	return nil
}

func (r *Repository) CreateMessageState(ctx context.Context, messageID string, state *pb.State) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO message_states (message_id, state, error, occurred_at)
		VALUES ($1, $2, $3, $4)`, messageID, state.State, state.Error, state.OccurredAt.AsTime())
	if err != nil {
		return fmt.Errorf("failed to insert message state: %w", err)
	}
	return nil
}

func (r *Repository) GetMessage(ctx context.Context, messageID string) (*pb.Message, []*pb.State, error) {
	msg := pb.Message{}
	states := []*pb.State{}
	err := r.db.QueryRow(ctx,
		`SELECT module, component, method, payload, 
		orchestration_name, orchestration_id, orchestration_step, orchestration_step_number, 
		orchestration_fallback_step FROM message WHERE id=$1`, messageID).Scan(
		&msg.Module, &msg.Component, &msg.Method, &msg.Payload,
		&msg.OrchestrationName, &msg.OrchestrationId, &msg.OrchestrationStep, &msg.OrchestrationStepNumber,
		&msg.OrchestrationFallbackStep)
	if err != nil {
		return nil, states, fmt.Errorf("failed to query message: %w", err)
	}

	stateRows, err := r.db.Query(ctx,
		`SELECT state, error, occurred_at FROM message_states WHERE message_id=$1
		ORDER BY occurred_at DESC`, messageID)
	if err != nil {
		return nil, states, fmt.Errorf("failed to query message: %w", err)
	}
	defer stateRows.Close()
	for stateRows.Next() {
		state := pb.State{}
		t := time.Time{}
		err := stateRows.Scan(&state.State, &state.Error, &t)
		if err != nil {
			return nil, states, fmt.Errorf("failed to scan message: %w", err)
		}
		state.OccurredAt = timestamppb.New(t)
		states = append(states, &state)
	}

	return &msg, states, nil
}
