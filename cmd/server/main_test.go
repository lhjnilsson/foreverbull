package main

import (
	"context"
	"net"
	"strconv"
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

	started := time.Now()
	err = pingPostgres(context.Background(), pool, 3, 10*time.Millisecond)

	require.Less(t, time.Since(started), 10*time.Second)
	require.ErrorContains(t, err, "127.0.0.1:"+strconv.Itoa(port))
	require.ErrorContains(t, err, "after 3 attempts")
	require.ErrorContains(t, err, "POSTGRES_URL")
	require.NotContains(t, err.Error(), "secret-password")
}
