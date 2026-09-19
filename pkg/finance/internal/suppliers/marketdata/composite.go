package marketdata

import (
	"errors"
	"fmt"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
)

// ErrYahooMetadataRequired is returned when MARKET_DATA_PROVIDER=stooq cannot
// construct Yahoo for GetAsset/SearchAsset/GetIndex. The process must fail closed.
var ErrYahooMetadataRequired = errors.New("MARKET_DATA_PROVIDER=stooq requires Yahoo for GetAsset/SearchAsset/GetIndex")

var _ supplier.Marketdata = (*Composite)(nil)

// Composite implements supplier.Marketdata with Stooq GetOHLC and Yahoo metadata.
type Composite struct {
	ohlc     supplier.Marketdata
	metadata supplier.Marketdata
}

// NewComposite returns a Marketdata that uses ohlc for GetOHLC and metadata for
// GetAsset/SearchAsset/GetIndex. Both clients are required (fail closed).
func NewComposite(ohlc, metadata supplier.Marketdata) (*Composite, error) {
	if ohlc == nil {
		return nil, errors.New("stooq composite: nil OHLC client")
	}
	if metadata == nil {
		return nil, fmt.Errorf("%w: constructor returned nil", ErrYahooMetadataRequired)
	}
	return &Composite{ohlc: ohlc, metadata: metadata}, nil
}

// NewStooqYahoo constructs Stooq for GetOHLC and Yahoo for metadata.
// If newYahoo is nil or fails, it fails closed and does not return an OHLC-only client.
func NewStooqYahoo(newYahoo func() (supplier.Marketdata, error), opts ...StooqOption) (supplier.Marketdata, error) {
	if newYahoo == nil {
		return nil, fmt.Errorf("%w: constructor is nil", ErrYahooMetadataRequired)
	}
	yahoo, err := newYahoo()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrYahooMetadataRequired, err)
	}
	stooq, err := NewStooqClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Stooq client: %w", err)
	}
	return NewComposite(stooq, yahoo)
}

func (c *Composite) GetAsset(symbol string) (*pb.Asset, error) {
	return c.metadata.GetAsset(symbol)
}

func (c *Composite) SearchAsset(q string) ([]*pb.Asset, error) {
	return c.metadata.SearchAsset(q)
}

func (c *Composite) GetIndex(symbol string) ([]*pb.Asset, error) {
	return c.metadata.GetIndex(symbol)
}

func (c *Composite) GetOHLC(symbol string, start time.Time, end *time.Time) ([]*pb.OHLC, error) {
	return c.ohlc.GetOHLC(symbol, start, end)
}
