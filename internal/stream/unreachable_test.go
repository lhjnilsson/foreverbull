package stream

import (
	"errors"
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

// An unreachable service rides on a command's state as its plain cause and
// comes back from the publisher as ErrServicesUnreachable.
func TestUnreachableStateRoundTrip(t *testing.T) {
	cause := Unreachable(fmt.Errorf("error storing session port: %w", errors.New("connection lost")))
	require.EqualError(t, cause, "error storing session port: connection lost")
	require.ErrorIs(t, cause, ErrServicesUnreachable)

	state := stateError(cause)
	require.EqualError(t, state, "foreverbull's services couldn't reach each other")
	require.ErrorIs(t, commandError(state.Error()), ErrServicesUnreachable)

	other := errors.New("error getting session: no rows")
	require.Equal(t, other, stateError(other))
	require.EqualError(t, commandError(other.Error()), "error getting session: no rows")

	require.EqualError(t, stateError(ErrTaskFinishedNotSaved), ErrTaskFinishedNotSaved.Error())
	got := commandError(ErrTaskFinishedNotSaved.Error())
	require.ErrorIs(t, got, ErrTaskFinishedNotSaved)
	require.EqualError(t, got, ErrTaskFinishedNotSaved.Error())

	require.EqualError(t, stateError(ErrTaskEndNotConfirmed), ErrTaskEndNotConfirmed.Error())
	got = commandError(ErrTaskEndNotConfirmed.Error())
	require.ErrorIs(t, got, ErrTaskEndNotConfirmed)
	require.EqualError(t, got, ErrTaskEndNotConfirmed.Error())
}

// A failed request is logged and stored. Scheme, host and path stay. The
// query and fragment do not, including a parameter that is not a crumb.
func TestCommandReportDropsRequestQuery(t *testing.T) {
	const raw = "https://www.ishares.com/us/products/239723/x.ajax?asOfDate=20200102&token=s3cr3tToken#frag"
	cause := fmt.Errorf("error doing request: %w", &url.Error{
		Op: "Get", URL: raw, Err: errors.New("dial tcp: connection refused"),
	})

	logged, stored := commandReport(cause)

	require.Equal(t, logged, stored)
	for _, err := range []error{logged, stored} {
		require.Contains(t, err.Error(), "https://www.ishares.com/us/products/239723/x.ajax")
		require.NotContains(t, err.Error(), "asOfDate")
		require.NotContains(t, err.Error(), "token=")
		require.NotContains(t, err.Error(), "s3cr3tToken")
		require.NotContains(t, err.Error(), "frag")
		require.NotContains(t, err.Error(), "?")
	}

	logged, stored = commandReport(Unreachable(cause))
	require.NotContains(t, logged.Error(), "s3cr3tToken")
	require.NotContains(t, logged.Error(), "asOfDate")
	require.ErrorIs(t, stored, ErrServicesUnreachable)
	require.NotContains(t, stored.Error(), "https://")
}
