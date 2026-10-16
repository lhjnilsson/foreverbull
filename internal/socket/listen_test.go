package socket

import (
	"errors"
	"net"
	"strconv"
	"sync"
	"syscall"
	"testing"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/stretchr/testify/require"
)

// freePortRange returns an inclusive range of count ports that were free a moment ago.
func freePortRange(t *testing.T, count int) environment.PortRange {
	t.Helper()

	for start := 41000; start+count <= 65000; start += count {
		listeners := make([]net.Listener, 0, count)
		free := true

		for port := start; port < start+count; port++ {
			listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
			if err != nil {
				free = false
				break
			}
			listeners = append(listeners, listener)
		}

		for _, listener := range listeners {
			require.NoError(t, listener.Close())
		}

		if free {
			return environment.PortRange{Start: start, End: start + count - 1}
		}
	}

	t.Fatalf("no run of %d free ports found", count)

	return environment.PortRange{}
}

func setPortRange(t *testing.T, portRange environment.PortRange) {
	t.Helper()

	t.Setenv(environment.BacktestPortRangeStart, strconv.Itoa(portRange.Start))
	t.Setenv(environment.BacktestPortRangeEnd, strconv.Itoa(portRange.End))
}

// An error that wraps nothing used to reach errors.Unwrap(err).Error() and
// crash on the nil result. It must come back as an error instead.
func TestListenInRangeReturnsUnwrappedErrorWithoutPanic(t *testing.T) {
	t.Parallel()

	portRange := environment.PortRange{Start: 27000, End: 27015}

	require.NotPanics(t, func() {
		_, err := ListenInRange(portRange, func(int) error { return errors.New("boom") })
		require.ErrorContains(t, err, "boom")
		require.ErrorContains(t, err, "port 27000")
	})
}

func TestListenInRangeSkipsBusyPortsAndUsesTheLastPort(t *testing.T) {
	t.Parallel()

	portRange := environment.PortRange{Start: 27000, End: 27002}
	var tried []int

	port, err := ListenInRange(portRange, func(port int) error {
		tried = append(tried, port)
		if port < portRange.End {
			return &net.OpError{Op: "listen", Err: syscall.EADDRINUSE}
		}
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, portRange.End, port)
	require.Equal(t, []int{27000, 27001, 27002}, tried)
}

func TestListenInRangeAllBusyNamesTheRange(t *testing.T) {
	t.Parallel()

	portRange := environment.PortRange{Start: 27000, End: 27001}

	_, err := ListenInRange(portRange, func(int) error {
		return &net.OpError{Op: "listen", Err: syscall.EADDRINUSE}
	})

	require.ErrorIs(t, err, ErrNoFreePort)
	require.ErrorContains(t, err, "27000-27001")
	require.ErrorContains(t, err, environment.BacktestPortRangeEnd)
}

// Sessions and trading listeners start concurrently. Each must hold its own
// port, and the range is exhausted only after every port, End included, is used.
func TestListenTCPConcurrentCallersGetDistinctPorts(t *testing.T) {
	const listenerCount = 8

	portRange := freePortRange(t, listenerCount)
	setPortRange(t, portRange)

	var wg sync.WaitGroup
	ports := make([]int, listenerCount)
	listeners := make([]net.Listener, listenerCount)
	errs := make([]error, listenerCount)

	for i := range listenerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			listeners[i], ports[i], errs[i] = ListenTCP("127.0.0.1")
		}()
	}
	wg.Wait()

	t.Cleanup(func() {
		for _, listener := range listeners {
			if listener != nil {
				_ = listener.Close()
			}
		}
	})

	seen := make(map[int]bool, listenerCount)
	for i := range listenerCount {
		require.NoError(t, errs[i])
		require.True(t, portRange.Contains(ports[i]), "port %d outside %v", ports[i], portRange)
		require.False(t, seen[ports[i]], "port %d handed out twice", ports[i])
		seen[ports[i]] = true
	}
	require.True(t, seen[portRange.End], "the last port in the range was never used")

	_, _, err := ListenTCP("127.0.0.1")
	require.ErrorIs(t, err, ErrNoFreePort)
}

func TestListenTCPRejectsBadPortRange(t *testing.T) {
	t.Setenv(environment.BacktestPortRangeStart, "27016")
	t.Setenv(environment.BacktestPortRangeEnd, "27015")

	_, _, err := ListenTCP("127.0.0.1")
	require.ErrorContains(t, err, environment.BacktestPortRangeStart)
}
