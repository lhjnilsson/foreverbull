package command

import (
	"context"
	"fmt"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/numfmt"
	"github.com/lhjnilsson/foreverbull/internal/stream"
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
// at least downloadProgressInterval old. The count never passes total.
func (p *downloadProgress) symbolDone() {
	if p.done < p.total {
		p.done++
	}
	if p.now().Sub(p.reported) >= downloadProgressInterval {
		p.report()
	}
}

func (p *downloadProgress) report() {
	p.reported = p.now()
	stream.ReportProgress(p.ctx, downloadProgressText(p.done, p.total))
}
