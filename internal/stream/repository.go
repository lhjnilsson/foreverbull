package stream

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type MessageStatus string

const (
	MessageStatusCreated   MessageStatus = "CREATED"
	MessageStatusPublished MessageStatus = "PUBLISHED"
	MessageStatusReceived  MessageStatus = "RECEIVED"
	MessageStatusComplete  MessageStatus = "COMPLETE"
	MessageStatusError     MessageStatus = "ERROR"
	MessageStatusCanceled  MessageStatus = "CANCELED"
)

func CreateTables(ctx context.Context, conn *pgxpool.Pool) error {
	if _, err := conn.Exec(context.Background(), `CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`); err != nil {
		return fmt.Errorf("failed to create extension: %w", err)
	}

	_, err := conn.Exec(ctx, table)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	return nil
}

func RecreateTables(ctx context.Context, conn *pgxpool.Pool) error {
	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS message_status;`); err != nil {
		return fmt.Errorf("failed to drop table: %w", err)
	}

	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS message;`); err != nil {
		return fmt.Errorf("failed to drop table: %w", err)
	}

	if _, err := conn.Exec(context.Background(), `CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`); err != nil {
		return fmt.Errorf("failed to create extension: %w", err)
	}

	_, err := conn.Exec(ctx, table)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	return nil
}

const table = `
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS message (
	id text PRIMARY KEY DEFAULT uuid_generate_v4 (),
	orchestration_name text,
	orchestration_id text,
	orchestration_step text,
	orchestration_step_number integer,
	orchestration_fallback_step boolean,

	module text NOT NULL,
	component text NOT NULL,
	method text NOT NULL,
	payload JSONB,

	status text NOT NULL DEFAULT 'CREATED',
	error text,

	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW());

CREATE TABLE IF NOT EXISTS message_status (
	id serial PRIMARY KEY,
	message_id text REFERENCES message(id) ON DELETE CASCADE,
	status text NOT NULL,
	error text,
	occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE OR REPLACE FUNCTION notify_message_status() RETURNS TRIGGER AS $$
BEGIN
	-- Only update message_status if the status column is updated
	IF (TG_OP = 'UPDATE' AND OLD.status <> NEW.status) OR TG_OP = 'INSERT' THEN
		INSERT INTO message_status (message_id, status, error)
		VALUES (NEW.id, NEW.status, NEW.error);
		PERFORM pg_notify('message_status', NEW.id);
	END IF;
	RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DO
$$BEGIN
	CREATE TRIGGER message_status_trigger AFTER INSERT OR UPDATE ON message
	FOR EACH ROW EXECUTE PROCEDURE notify_message_status();
EXCEPTION
	WHEN duplicate_object THEN
		NULL;
END$$;
`

type repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) repository {
	return repository{db: db}
}

func (r *repository) CreateMessage(ctx context.Context, msg *message) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO message (orchestration_name, orchestration_id, orchestration_step, orchestration_step_number,
			orchestration_fallback_step, module, component, method, payload)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`, msg.OrchestrationName,
		msg.OrchestrationID, msg.OrchestrationStep, msg.OrchestrationStepNumber, msg.OrchestrationFallbackStep,
		msg.Module, msg.Component, msg.Method, msg.Payload).Scan(&msg.ID)
	if err != nil {
		return fmt.Errorf("failed to insert message: %w", err)
	}

	return nil
}

func (r *repository) GetMessage(ctx context.Context, messageID string) (*message, error) {
	m := message{}

	rows, err := r.db.Query(ctx,
		`SELECT message.id, orchestration_name, orchestration_id, orchestration_step, orchestration_step_number, orchestration_fallback_step,
		module, component, method, payload, ms.status, ms.error, ms.occurred_at
		FROM message
		INNER JOIN (
			SELECT message_id, status, error, occurred_at FROM message_status ORDER BY occurred_at DESC
		)	AS ms ON message.id=ms.message_id
		WHERE message.id=$1`, messageID)
	if err != nil {
		return nil, fmt.Errorf("failed to query message: %w", err)
	}

	defer rows.Close()

	for rows.Next() {
		status := messageStatus{}

		err := rows.Scan(&m.ID, &m.OrchestrationName, &m.OrchestrationID, &m.OrchestrationStep, &m.OrchestrationStepNumber, &m.OrchestrationFallbackStep,
			&m.Module, &m.Component, &m.Method, &m.Payload, &status.Status, &status.Error, &status.OccurredAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}

		m.StatusHistory = append(m.StatusHistory, status)
	}

	return &m, nil
}

func (r *repository) UpdatePublishedAndGetMessage(ctx context.Context, messageID string) (*message, error) {
	var msgID *string

	err := r.db.QueryRow(ctx,
		`UPDATE message SET status=$1 WHERE id=$2 AND status=$3
		RETURNING id`, MessageStatusReceived, messageID, MessageStatusPublished).Scan(&msgID)
	if err != nil {
		return nil, fmt.Errorf("failed to update message: %w", err)
	}

	return r.GetMessage(ctx, *msgID)
}

func (r *repository) UpdateMessageStatus(ctx context.Context, messageID string, status MessageStatus, err error) error {
	if err != nil {
		_, err = r.db.Exec(ctx,
			`UPDATE message SET status=$1, error=$2 WHERE id=$3`,
			status, err.Error(), messageID)
	} else {
		_, err = r.db.Exec(ctx,
			`UPDATE message SET status=$1 WHERE id=$2`,
			status, messageID)
	}

	if err != nil {
		return fmt.Errorf("failed to update message status: %w", err)
	}

	return nil
}

func (r *repository) OrchestrationIsRunning(ctx context.Context, orchestrationID string) (bool, error) {
	var total int

	var completed int

	var canceled int

	var created int

	err := r.db.QueryRow(ctx,
		`SELECT total.num, completed.num, canceled.num, created.num
		FROM (
			SELECT COUNT(*) as num FROM message WHERE message.orchestration_id=$1
		) as total
		INNER JOIN (
			SELECT COUNT(*) as num FROM message WHERE message.orchestration_id=$1 AND message.status=$2
		) as completed ON true
		INNER JOIN (
			SELECT COUNT(*) as num FROM message WHERE message.orchestration_id=$1 AND message.status=$3
		) as canceled ON true
		INNER JOIN (
			SELECT COUNT(*) as num FROM message WHERE message.orchestration_id=$1 AND message.status=$4
		) as created ON true`, orchestrationID, MessageStatusComplete, MessageStatusCanceled, MessageStatusCreated).Scan(
		&total, &completed, &canceled, &created)
	if err != nil {
		return false, fmt.Errorf("failed to query orchestration status: %w", err)
	}

	if (total == created) || (total == (completed + canceled)) {
		return false, nil
	}

	return true, nil
}

func (r *repository) OrchestrationIsComplete(ctx context.Context, orchestrationID string) (bool, error) {
	var count int

	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM message WHERE message.orchestration_id=$1 AND message.status IN ($2, $3, $4)
		AND message.orchestration_fallback_step=false`,
		orchestrationID, MessageStatusCreated, MessageStatusPublished, MessageStatusReceived).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to query orchestration status: %w", err)
	}

	return !(count > 0), nil
}

func (r *repository) GetNextOrchestrationCommands(ctx context.Context, orchestrationID string, currentStepNumber int) (*[]message, error) {
	var stepComplete bool

	err := r.db.QueryRow(ctx,
		`SELECT count(*) = count(*) filter (where status IN ($1, $2)) FROM message WHERE orchestration_id=$3 AND orchestration_step_number=$4`,
		MessageStatusComplete, MessageStatusError, orchestrationID, currentStepNumber).Scan(&stepComplete)
	if err != nil {
		return nil, fmt.Errorf("failed to query orchestration status: %w", err)
	}

	var msgs []message
	if !stepComplete {
		return &msgs, nil
	}

	rows, err := r.db.Query(ctx, `
WITH orchestration AS (
	SELECT id, orchestration_id, orchestration_step, orchestration_step_number, orchestration_fallback_step, status, module, component, method, payload FROM message WHERE message.orchestration_id=$1
) SELECT id, orchestration_id, orchestration_step, orchestration_fallback_step, module, component, method, payload FROM orchestration WHERE
CASE
    WHEN (SELECT EXISTS(SELECT 1 FROM orchestration WHERE status = $2)) THEN
        orchestration_fallback_step=true
    ELSE
        orchestration_fallback_step=false and orchestration.orchestration_step_number=$3 and orchestration.status=$4
END`, orchestrationID, MessageStatusError, currentStepNumber+1, MessageStatusCreated)
	if err != nil {
		return nil, fmt.Errorf("failed to query orchestration commands: %w", err)
	}

	defer rows.Close()

	for rows.Next() {
		m := message{}

		err := rows.Scan(&m.ID, &m.OrchestrationID, &m.OrchestrationStep, &m.OrchestrationFallbackStep, &m.Module, &m.Component, &m.Method, &m.Payload)
		if err != nil {
			return nil, fmt.Errorf("failed to scan orchestration commands: %w", err)
		}

		msgs = append(msgs, m)
	}

	return &msgs, nil
}

func (r *repository) OrchestrationStepIsComplete(ctx context.Context, orchestrationID, step string) (bool, error) {
	var count int

	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM message WHERE message.orchestration_id=$1 AND message.orchestration_step=$2
		AND message.status IN ($3, $4, $5)`,
		orchestrationID, step, MessageStatusCreated, MessageStatusPublished, MessageStatusReceived).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to query orchestration status: %w", err)
	}

	return !(count > 0), nil
}

func (r *repository) OrchestrationHasError(ctx context.Context, orchestrationID string) (bool, error) {
	var count int

	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM message WHERE message.orchestration_id=$1 AND message.error IS NOT NULL`,
		orchestrationID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to query orchestration status: %w", err)
	}

	return count > 0, nil
}

func (r *repository) MarkAllCreatedAsCanceled(ctx context.Context, orchestrationID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE message SET status=$1 WHERE message.orchestration_id=$2 AND message.status IN ($3)`,
		MessageStatusCanceled, orchestrationID, MessageStatusCreated)
	if err != nil {
		return fmt.Errorf("failed to mark all created messages as canceled: %w", err)
	}

	return nil
}
