package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNoISINMessage(t *testing.T) {
	t.Parallel()

	require.Equal(t, "no ISIN for XYZ", NoISINMessage("XYZ"))
	require.True(t, IsNoISIN(NoISINMessage("XYZ")))
	require.False(t, IsNoISIN("no price data for XYZ"))
	require.False(t, IsNoISIN("Something went wrong. no ISIN for XYZ"))
}
