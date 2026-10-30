package command

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Only the finance service's exact refusal sentence reaches the user; anything
// else falls back to the generic download sentence.
func TestRefusedPriceErrorOnlyMatchesTheRefusalSentence(t *testing.T) {
	const tail = " sent an invalid price for XYZ on 2026-05-02 (0, negative or not a number)"
	for _, provider := range []string{"yahoo", "stooq", "alpaca"} {
		require.True(t, refusedPriceError.MatchString(provider+tail), provider)
	}
	require.True(t, refusedPriceError.MatchString(
		"yahoo sent an invalid price for BRK.B on 2026-05-02 (0, negative or not a number)"))

	for name, text := range map[string]string{
		"unknown provider":  "postgres" + tail,
		"escape in symbol":  "yahoo sent an invalid price for X\x1b[2JZ on 2026-05-02 (0, negative or not a number)",
		"symbol too long":   "yahoo sent an invalid price for ABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFG on 2026-05-02 (0, negative or not a number)",
		"trailing raw text": "yahoo" + tail + ": dial tcp 10.0.0.1:5432",
		"leading raw text":  "error: yahoo" + tail,
		"bad date":          "yahoo sent an invalid price for XYZ on 05/02/2026 (0, negative or not a number)",
	} {
		require.False(t, refusedPriceError.MatchString(text), name)
	}
}

// The out-of-range volume sentence is a refused bar too; the price regex must
// not accept it and vice versa.
func TestRefusedVolumeErrorOnlyMatchesTheRefusalSentence(t *testing.T) {
	const tail = " sent an invalid volume for XYZ on 2026-05-02 (out of range)"
	for _, provider := range []string{"yahoo", "stooq", "alpaca"} {
		require.True(t, refusedVolumeError.MatchString(provider+tail), provider)
		require.True(t, isRefusedBarError(provider+tail), provider)
		require.False(t, refusedPriceError.MatchString(provider+tail), provider)
	}
	require.True(t, isRefusedBarError("yahoo sent an invalid price for XYZ on 2026-05-02 (0, negative or not a number)"))

	for name, text := range map[string]string{
		"unknown provider":    "postgres" + tail,
		"escape in symbol":    "yahoo sent an invalid volume for X\x1b[2JZ on 2026-05-02 (out of range)",
		"symbol too long":     "yahoo sent an invalid volume for ABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFG on 2026-05-02 (out of range)",
		"trailing raw text":   "yahoo" + tail + ": dial tcp 10.0.0.1:5432",
		"leading raw text":    "error: yahoo" + tail,
		"bad date":            "yahoo sent an invalid volume for XYZ on 05/02/2026 (out of range)",
		"price reason":        "yahoo sent an invalid volume for XYZ on 2026-05-02 (0, negative or not a number)",
		"volume reason price": "yahoo sent an invalid price for XYZ on 2026-05-02 (out of range)",
	} {
		require.False(t, isRefusedBarError(text), name)
	}
}
