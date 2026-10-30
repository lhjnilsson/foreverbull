package api

import (
	"sync"
	"sync/atomic"
	"testing"

	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

// countingStream fails the test when two Sends overlap. Its unguarded append
// also trips -race if Send is ever called concurrently.
type countingStream struct {
	grpc.ServerStream

	inFlight atomic.Int32
	overlaps atomic.Int32
	sent     []*servicePb.DownloadHistoricalDataResponse
}

func (s *countingStream) Send(rsp *servicePb.DownloadHistoricalDataResponse) error {
	if s.inFlight.Add(1) > 1 {
		s.overlaps.Add(1)
	}
	defer s.inFlight.Add(-1)
	s.sent = append(s.sent, rsp)
	return nil
}

func TestDownloadSenderSerialisesSend(t *testing.T) {
	stream := &countingStream{}
	sender := &downloadSender{stream: stream}

	const workers, perWorker = downloadWorkers, 50
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range perWorker {
				sender.send(&servicePb.DownloadHistoricalDataResponse{Symbol: "X"})
			}
		}()
	}
	wg.Wait()

	assert.Zero(t, stream.overlaps.Load(), "Send must never run concurrently")
	assert.Len(t, stream.sent, workers*perWorker)
}
