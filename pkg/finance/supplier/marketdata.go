package supplier

import (
	"time"

	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
)

// DefaultProvider is the price source for a backtest whose provider is empty.
// It is the one Go place to change that default. The Python client keeps its
// own DEFAULT_PROVIDER in client/src/foreverbull/models/finance.py, and both
// must name the same provider.
const DefaultProvider = "stooq"

type Marketdata interface {
	GetAsset(symbol string) (*pb.Asset, error)
	SearchAsset(q string) ([]*pb.Asset, error)
	GetIndex(symbol string) ([]*pb.Asset, error)
	GetOHLC(symbol string, start time.Time, end *time.Time) ([]*pb.OHLC, error)
}
