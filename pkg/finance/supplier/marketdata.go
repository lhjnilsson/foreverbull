package supplier

import (
	"context"
	"time"

	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
)

// DefaultProvider is the price source used when a stored row or a worker
// environment has no provider. Create does not substitute it. The Python
// client keeps DEFAULT_PROVIDER in client/src/foreverbull/models/finance.py
// for an unset OHLC_PROVIDER, and both must name the same provider.
const DefaultProvider = "yahoo"

// Marketdata is a price and metadata provider. Every call takes the caller's
// ctx, so a cancelled request stops the provider fetch too.
type Marketdata interface {
	GetAsset(ctx context.Context, symbol string) (*pb.Asset, error)
	SearchAsset(ctx context.Context, q string) ([]*pb.Asset, error)
	GetIndex(ctx context.Context, symbol string) ([]*pb.Asset, error)
	GetOHLC(ctx context.Context, symbol string, start time.Time, end *time.Time) ([]*pb.OHLC, error)
}
