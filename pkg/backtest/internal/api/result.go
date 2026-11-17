package api

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/internal/auth"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/internal/usercopy"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest/service"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// resultChunkBytes is one streamed piece of a stored execution result.
const resultChunkBytes = 1 << 20

// ResultReader opens the stored Zipline result for one execution.
// The caller closes the reader. Implementations must not return a storage
// address or a credential.
type ResultReader interface {
	ReadResult(ctx context.Context, executionID string) (io.ReadCloser, int64, error)
}

// GetExecutionResult streams the stored result for an execution the caller
// may read. The bytes are the result. Nothing in the stream is a storage URL.
func (bs *BacktestServer) GetExecutionResult(
	req *servicePb.GetExecutionResultRequest,
	stream servicePb.BacktestServicer_GetExecutionResultServer,
) error {
	ctx := stream.Context()
	id := req.GetExecutionId()
	if !storage.ResultKey(id) {
		return status.Error(codes.InvalidArgument, usercopy.ExecutionIDInvalid())
	}
	if err := bs.allowExecutionResult(ctx, id); err != nil {
		return err
	}
	if bs.results == nil {
		return errors.New("execution results are not available")
	}
	body, size, err := bs.results.ReadResult(ctx, id)
	if err != nil {
		return resultReadError(err)
	}
	defer func() {
		if closeErr := body.Close(); closeErr != nil {
			log.Error().Err(closeErr).Str("execution", id).Msg("close execution result")
		}
	}()
	if size > storage.ResultMaxBytes {
		return status.Error(codes.ResourceExhausted, usercopy.ExecutionResultTooLarge())
	}
	return streamResult(ctx, size, body, stream)
}

// allowExecutionResult is the same read check GetExecution uses.
// Auth off still requires the execution to exist, so an arbitrary key is not read.
func (bs *BacktestServer) allowExecutionResult(ctx context.Context, executionID string) error {
	executions := repository.Execution{Conn: bs.pgx}
	if environment.GetAuthEnabled() {
		account, err := accountFrom(ctx)
		if err != nil {
			return err
		}
		backtestID, err := executions.BacktestID(ctx, executionID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return status.Error(codes.NotFound, "not found")
			}
			return fmt.Errorf("error getting execution: %w", err)
		}
		if err := auth.Can(ctx, bs.pgx, account, auth.DomainBacktest, backtestID, auth.ActionRead); err != nil {
			return auth.GRPCStatus(err)
		}
		return nil
	}
	if _, err := executions.Get(ctx, executionID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return status.Error(codes.NotFound, "not found")
		}
		return fmt.Errorf("error getting execution: %w", err)
	}
	return nil
}

func resultReadError(err error) error {
	switch {
	case errors.Is(err, storage.ErrResultMissing):
		return status.Error(codes.FailedPrecondition, usercopy.ExecutionResultMissing())
	case errors.Is(err, storage.ErrResultTooLarge):
		return status.Error(codes.ResourceExhausted, usercopy.ExecutionResultTooLarge())
	default:
		return fmt.Errorf("read execution result: %w", err)
	}
}

func streamResult(ctx context.Context, size int64, body io.Reader, stream servicePb.BacktestServicer_GetExecutionResultServer) error {
	buf := make([]byte, resultChunkBytes)
	var sent int64
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, err := body.Read(buf)
		if n == 0 && err == nil {
			return errors.New("read execution result: empty read")
		}
		if n > 0 {
			sent += int64(n)
			if sent > storage.ResultMaxBytes {
				return status.Error(codes.ResourceExhausted, usercopy.ExecutionResultTooLarge())
			}
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			if sendErr := stream.Send(&servicePb.GetExecutionResultResponse{Data: chunk}); sendErr != nil {
				return sendErr
			}
		}
		if errors.Is(err, io.EOF) {
			if sent == 0 {
				return status.Error(codes.FailedPrecondition, usercopy.ExecutionResultMissing())
			}
			if size > 0 && sent != size {
				return fmt.Errorf("read execution result: got %d bytes, object is %d", sent, size)
			}
			return nil
		}
		if err != nil {
			return fmt.Errorf("read execution result: %w", err)
		}
	}
}
