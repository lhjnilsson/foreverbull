package supplier

import (
	"time"

	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
)

type Marketdata interface {
	GetAsset(symbol string) (*pb.Asset, error)
	GetIndex(symbol string) ([]*pb.Asset, error)
	GetOHLC(symbol string, start time.Time, end *time.Time) ([]*pb.OHLC, error)
}
