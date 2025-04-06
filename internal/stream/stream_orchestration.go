package stream

import (
	"context"

	"github.com/lhjnilsson/foreverbull/internal/stream/pb"
	commonPb "github.com/lhjnilsson/foreverbull/pkg/pb"
	"golang.org/x/sync/errgroup"
)

func (s *NATSStream) runOrchestrationStep(ctx context.Context, step *pb.Orchestration_Step) error {
	g, ctx := errgroup.WithContext(ctx)
	for _, command := range step.Messages {
		g.Go(func() error {
			msg := &message{msg: command}
			return s.Publish(ctx, msg, WaitForCompletion())
		})
	}
	return g.Wait()
}

func (s *NATSStream) RunOrchestration(ctx context.Context, o *pb.Orchestration) <-chan *pb.Orchestration_Step {
	out := make(chan *pb.Orchestration_Step)

	go func() {
		defer close(out)
		for _, step := range o.Steps {
			runningStep := pb.Orchestration_Step{Status: step.Status, State: commonPb.State_STATE_RUNNING}

			out <- &runningStep
			err := s.runOrchestrationStep(ctx, step)
			if err != nil {
				errStr := err.Error()
				failedStep := pb.Orchestration_Step{Status: step.Status, State: commonPb.State_STATE_FAILED, Error: &errStr}
				out <- &failedStep
				return
			} else {
				completedStep := pb.Orchestration_Step{Status: step.Status, State: commonPb.State_STATE_COMPLETED}
				out <- &completedStep
			}
		}
	}()
	return out
}
