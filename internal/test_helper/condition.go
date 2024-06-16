package test_helper

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func WaitUntilCondition(t *testing.T, condition func() (bool, error), timeout time.Duration) error {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for condition: %w", ctx.Err())
		default:
			ok, err := condition()
			if err != nil {
				return err
			}
			if !ok {
				time.Sleep(time.Second / 4)
				continue
			}
			return nil
		}
	}
}
