package main

import (
	"bytes"
	"context"
	"log"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// A POSTGRES_URL that points at nothing must fail in bounded time with an
// error that names the host, and must not leak the password.
func TestPingPostgresGivesUpAndNamesHost(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())

	url := "postgres://postgres:secret-password@127.0.0.1:" + strconv.Itoa(port) + "/postgres?sslmode=disable"
	pool, err := pgxpool.New(context.Background(), url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })

	started := time.Now()
	err = pingPostgres(context.Background(), pool, 3, 10*time.Millisecond)

	require.Less(t, time.Since(started), 10*time.Second)
	require.ErrorContains(t, err, "127.0.0.1:"+strconv.Itoa(port))
	require.ErrorContains(t, err, "after 3 attempts")
	require.ErrorContains(t, err, "POSTGRES_URL")
	require.NotContains(t, err.Error(), "secret-password")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.Len(t, lines, 2)
	require.Contains(t, lines[0], "attempt 1 of 3")
	require.Contains(t, lines[0], "if this persists, check POSTGRES_URL and that Postgres is running")
	require.Contains(t, lines[1], "attempt 2 of 3")
	require.NotContains(t, lines[1], "if this persists")
	require.NotContains(t, buf.String(), "secret-password")
}
