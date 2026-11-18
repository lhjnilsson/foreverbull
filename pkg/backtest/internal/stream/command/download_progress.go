package command

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/numfmt"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	financeServicePB "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
)

// downloadProgressInterval is the shortest gap between two progress
// reports. Reports go over core NATS and aren't stored, but a waiter still
// handles each one, so an hour-long download sends at most 1,800, and the
// CLI line still moves often enough not to look stuck.
const downloadProgressInterval = 2 * time.Second

// downloadProgress reports "Downloading prices: N of TOTAL symbols" while
// DownloadFinancialData reads the finance service's responses. total is the
// distinct symbol count the finance service answers for, once each.
type downloadProgress struct {
	ctx      context.Context //nolint:containedctx // the reporter rides on the command's ctx
	total    int
	done     int
	reported time.Time
	now      func() time.Time
	// provider and waitSeconds are the pause the current line names.
	// Empty once a symbol result continues the download.
	provider    string
	waitSeconds int32
}

func newDownloadProgress(ctx context.Context, total int) *downloadProgress {
	return &downloadProgress{ctx: ctx, total: total, now: time.Now}
}

// downloadProgressText is the agreed progress line, word for word.
func downloadProgressText(done, total int) string {
	return fmt.Sprintf("Downloading prices: %s of %s symbols", numfmt.Thousands(done), numfmt.Thousands(total))
}

// start reports 0 of total, so the line shows before the first symbol ends.
func (p *downloadProgress) start() {
	p.report()
}

// symbolDone counts one answered symbol and reports when the last report is
// at least downloadProgressInterval old, or at once when the line was naming
// a provider pause. That next line has no pause, so the note is cleared.
// The count never passes total.
func (p *downloadProgress) symbolDone() {
	if p.done < p.total {
		p.done++
	}
	waiting := p.waitSeconds > 0
	p.provider = ""
	p.waitSeconds = 0
	if waiting || p.now().Sub(p.reported) >= downloadProgressInterval {
		p.report()
	}
}

// noteRateLimit names a provider pause on the current line and reports it
// at once, ahead of downloadProgressInterval. provider is the name finance
// sent, whatever it is. seconds is the whole seconds of the pause.
func (p *downloadProgress) noteRateLimit(provider string, seconds int32) error {
	if seconds <= 0 {
		return fmt.Errorf("price download named a rate-limit wait of %d s", seconds)
	}
	p.provider = provider
	p.waitSeconds = seconds
	p.report()
	return nil
}

func (p *downloadProgress) report() {
	p.reported = p.now()
	text := downloadProgressText(p.done, p.total)
	if p.waitSeconds > 0 {
		stream.ReportRateLimit(p.ctx, text, p.provider, p.waitSeconds)
		return
	}
	stream.ReportProgress(p.ctx, text)
}

// rateLimitPause is the provider pause on msg when that frame is only a
// pause. A symbol result is not one. A frame that carries both, or a
// non-positive wait, is an error. The provider name is passed through.
func rateLimitPause(msg *financeServicePB.DownloadHistoricalDataResponse) (provider string, seconds int32, ok bool, err error) {
	wait := msg.GetRateLimit()
	if wait == nil {
		return "", 0, false, nil
	}
	if msg.GetSymbol() != "" ||
		msg.GetStatus() != financeServicePB.DownloadHistoricalDataResponse_STATUS_UNSPECIFIED ||
		msg.GetError() != "" ||
		msg.GetProviderStatus() != nil ||
		msg.GetLaterTicker() != nil ||
		msg.GetDividendFailure() != nil {
		return "", 0, false, errors.New("price download sent a rate-limit pause on a symbol result")
	}
	if wait.GetWaitSeconds() <= 0 {
		return "", 0, false, fmt.Errorf("price download named a rate-limit wait of %d s", wait.GetWaitSeconds())
	}
	return wait.GetProvider(), wait.GetWaitSeconds(), true, nil
}
