package http

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	"github.com/stretchr/testify/suite"
)

const table = `CREATE TABLE IF NOT EXISTS http_test (
	id SERIAL PRIMARY KEY,
	not_null text NOT NULL,
	uniq text UNIQUE,
	min_five_length text CHECK (LENGTH(min_five_length) > 5));`

type HTTPTest struct {
	suite.Suite
	Conn *pgxpool.Pool
}

func (h *HTTPTest) SetupTest() {
	config := helper.TestingConfig(h.T(), &helper.Containers{
		Postgres: true,
	})
	pool, err := pgxpool.New(context.Background(), config.PostgresURI)
	h.NoError(err)

	h.Conn = pool
	pool.Exec(context.Background(), "DROP TABLE IF EXISTS http_test")
	_, err = pool.Exec(context.Background(), table)
	h.NoError(err)
}

func (h *HTTPTest) TearDownTest() {
	h.Conn.Close()
}

func TestDatabaseError(t *testing.T) {
	suite.Run(t, new(HTTPTest))
}

func (h *HTTPTest) TestSetNull() {
	_, err := h.Conn.Exec(context.Background(),
		`INSERT INTO http_test (not_null, uniq, min_five_length)
		VALUES (NULL, 'unique', 'min_five_length')`)
	h.Error(err)
	code, apiError := DatabaseError(err)
	h.Equal(400, code)
	h.Equal("Value cant be null", apiError.Message)
}

func (h *HTTPTest) TestDuplicateEntry() {
	_, err := h.Conn.Exec(context.Background(),
		`INSERT INTO http_test (not_null, uniq, min_five_length)
		VALUES ('not_null', 'unique', 'min_five_length')`)
	h.NoError(err)
	_, err = h.Conn.Exec(context.Background(),
		`INSERT INTO http_test (not_null, uniq, min_five_length)
		VALUES ('not_null', 'unique', 'min_five_length')
		RETURNING id`)
	h.Error(err)
	code, apiError := DatabaseError(err)
	h.Equal(409, code)
	h.Equal("Conflict", apiError.Message)
}

func (h *HTTPTest) TestNameTooShort() {
	_, err := h.Conn.Exec(context.Background(),
		`INSERT INTO http_test (not_null, uniq, min_five_length)
		VALUES ('not_null', 'unique', 'short')`)
	h.Error(err)
	code, apiError := DatabaseError(err)
	h.Equal(400, code)
	h.Equal("Value does not meet requirements", apiError.Message)
}

func (h *HTTPTest) TestNotFound() {
	err := h.Conn.QueryRow(context.Background(),
		`SELECT * FROM http_test WHERE id=1`).Scan()
	code, apiError := DatabaseError(err)
	h.Equal(404, code)
	h.Equal("Not Found", apiError.Message)
}
