package api

import (
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/marketdata"
	internal_pb "github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/stretchr/testify/require"
)

func TestSessionCutoffAsOfRefusesANonDate(t *testing.T) {
	t.Parallel()
	for _, day := range []*internal_pb.Date{
		nil,
		{},
		{Year: 2024, Month: 2, Day: 31},
		{Year: 2024, Month: 0, Day: 1},
	} {
		_, err := sessionCutoffAsOf(day)
		require.EqualError(t, err, "session_cutoff is not a date")
	}
	asOf, err := sessionCutoffAsOf(&internal_pb.Date{Year: 2024, Month: 3, Day: 27})
	require.NoError(t, err)
	require.True(t, asOf.Equal(marketdata.SessionAsOf(time.Date(2024, 3, 27, 0, 0, 0, 0, time.UTC))))
}
