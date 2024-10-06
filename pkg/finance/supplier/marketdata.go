package supplier

import (
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/finance/pb"
)

type Marketdata interface {
	GetAsset(symbol string) (*pb.Asset, error)
	GetIndex(symbol string) ([]*pb.Asset, error)
	GetOHLC(symbol string, start, end time.Time) ([]*pb.OHLC, error)
}
