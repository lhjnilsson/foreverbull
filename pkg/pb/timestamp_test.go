package pb

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestDateConversionRoundTrip(t *testing.T) {
	t.Parallel()

	src := time.Date(2024, time.March, 9, 15, 4, 5, 0, time.UTC)
	date := GoTimeToDate(src)

	require.Equal(t, int32(2024), date.Year)
	require.Equal(t, int32(3), date.Month)
	require.Equal(t, int32(9), date.Day)
	require.Equal(t, "2024-03-09", DateToDateString(date))
	require.Equal(t, date, DateStringToDate("2024-03-09"))
	require.True(t, DateToTime(date).Equal(time.Date(2024, time.March, 9, 0, 0, 0, 0, time.UTC)))
	require.True(t, timestamppb.New(src).AsTime().Equal(TimeToProtoTimestamp(src).AsTime()))
}

func TestDateStringToDateInvalidAndNil(t *testing.T) {
	t.Parallel()

	require.Empty(t, DateToDateString(nil))

	cases := []string{"abc-03-09", "2024-xx-09", "2024-03-yy"}
	for _, input := range cases {
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			got := DateStringToDate(input)
			require.Equal(t, &Date{Year: 0, Month: 0, Day: 0}, got)
		})
	}
}
