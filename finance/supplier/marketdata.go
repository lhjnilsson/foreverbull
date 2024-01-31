package supplier

import (
	"time"

	"github.com/lhjnilsson/foreverbull/finance/entity"
)

type Marketdata interface {
	GetAsset(symbol string) (*entity.Asset, error)
	GetOHLC(symbol string, start, end time.Time) (*[]entity.OHLC, error)
}
