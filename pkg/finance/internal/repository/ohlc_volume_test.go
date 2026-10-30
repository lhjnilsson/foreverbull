package repository_test

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func volumeBar(day time.Time, volume uint64) *pb.OHLC {
	return &pb.OHLC{
		Timestamp: timestamppb.New(day),
		Open:      1.2, High: 1.3, Low: 1.1, Close: 1.25,
		Volume: volume,
	}
}

// A volume above ohlc.volume (bigint) refuses the whole batch with its own
// sentence, before anything is stored.
func TestValidateBarsRejectsOutOfRangeVolume(t *testing.T) {
	day := time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)
	for _, volume := range []uint64{uint64(math.MaxInt64) + 1, math.MaxUint64} {
		err := repository.ValidateBars("yahoo", "ABC", []*pb.OHLC{
			volumeBar(day.AddDate(0, 0, -1), 100),
			volumeBar(day, volume),
		})
		require.ErrorIs(t, err, repository.ErrInvalidPrice)
		require.NotErrorIs(t, err, supplier.ErrRejected)
		require.EqualError(t, err, "yahoo sent an invalid volume for ABC on 2020-01-02 (out of range)")
		var refused *repository.InvalidPriceError
		require.ErrorAs(t, err, &refused)
		require.Equal(t, "volume", refused.Field)
	}
	require.NoError(t, repository.ValidateBars("yahoo", "ABC", []*pb.OHLC{volumeBar(day, math.MaxInt64)}))
}

// Store refuses an out-of-range volume before it touches the connection.
func TestStoreRejectsOutOfRangeVolume(t *testing.T) {
	db := repository.OHLC{}
	day := time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)
	err := db.Store(context.Background(), "stooq", "ABC", day, 1.2, 1.3, 1.1, 1.25, uint64(math.MaxInt64)+1)
	require.ErrorIs(t, err, repository.ErrInvalidPrice)
	require.NotErrorIs(t, err, supplier.ErrRejected)
	require.EqualError(t, err, "stooq sent an invalid volume for ABC on 2020-01-02 (out of range)")
}

// A bad price is reported before a bad volume on the same bar.
func TestValidateBarsReportsPriceBeforeVolume(t *testing.T) {
	day := time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)
	bar := volumeBar(day, math.MaxUint64)
	bar.Close = 0
	err := repository.ValidateBars("yahoo", "ABC", []*pb.OHLC{bar})
	require.EqualError(t, err, "yahoo sent an invalid price for ABC on 2020-01-02 (0, negative or not a number)")
}

func (test *OHLCTests) TestStoreMaxVolume() {
	day := time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)
	test.Require().NoError(test.ohlcStorage.Store(context.Background(), "yahoo", "ABC", day, 1.2, 1.3, 1.1, 1.25, math.MaxInt64))
	var v int64
	test.Require().NoError(test.conn.QueryRow(context.Background(),
		`SELECT volume FROM ohlc WHERE symbol = 'ABC'`).Scan(&v))
	test.Equal(int64(math.MaxInt64), v)
}
