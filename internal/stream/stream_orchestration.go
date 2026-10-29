package stream

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/lhjnilsson/foreverbull/internal/stream/pb"
	commonPb "github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

// runOrchestrationStep returns the notices of commands that completed with
// one. A notice is not an error, so it must not cancel the step's other
// commands through the errgroup context.
func (s *NATSStream) runOrchestrationStep(ctx context.Context, step *pb.Orchestration_Step) ([]string, error) {
	g, ctx := errgroup.WithContext(ctx)
	var mu sync.Mutex
	notices := make([]string, 0)
	for _, command := range step.Messages {
		g.Go(func() error {
			msg := &message{msg: command}
			err := s.Publish(ctx, msg, WaitForCompletion())
			var notice *Notice
			if errors.As(err, &notice) {
				mu.Lock()
				notices = append(notices, notice.Text)
				mu.Unlock()
				return nil
			}
			return err
		})
	}
	return notices, g.Wait()
}

func (s *NATSStream) RunOrchestration(ctx context.Context, o *pb.Orchestration) <-chan *pb.Orchestration_Step {
	out := make(chan *pb.Orchestration_Step)

	go func() {
		defer close(out)
		for _, step := range o.Steps {
			runningStep := pb.Orchestration_Step{Num: step.Num, State: commonPb.State_STATE_RUNNING}

			out <- &runningStep
			notices, err := s.runOrchestrationStep(ctx, step)
			if err != nil {
				log.Warn().Err(err).Msg("error running orchestration step")
				errStr := err.Error()
				failedStep := pb.Orchestration_Step{Num: step.Num, State: commonPb.State_STATE_FAILED, Error: &errStr}
				out <- &failedStep
				return
			}
			completedStep := pb.Orchestration_Step{Num: step.Num, State: commonPb.State_STATE_COMPLETED}
			if len(notices) > 0 {
				// UpdateOrchestration copies Error into the step's message.
				text := strings.Join(notices, "\n")
				completedStep.Error = &text
			}
			out <- &completedStep
		}
	}()
	return out
}
