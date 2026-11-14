package runner

import (
	"context"
	"errors"
	"testing"

	pb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy"
	"github.com/stretchr/testify/require"
)

// ERROR replaces a RUNNING deployment and stores the message. The same write
// on a STOPPED deployment returns nil and leaves the status and error. A
// status other than ERROR is still written with no status check.
func TestMemoryStoreUpdateStatusErrorOnlyWhileRunning(t *testing.T) {
	ctx := context.Background()

	t.Run("error over running", func(t *testing.T) {
		store := NewMemoryStore()
		store.Put(&pb.Deployment{Id: "dep-1", Status: pb.Deployment_RUNNING})

		err := store.UpdateStatus(ctx, "dep-1", pb.Deployment_ERROR, errors.New("clock failed"))
		require.NoError(t, err)

		got, err := store.GetDeployment(ctx, "dep-1")
		require.NoError(t, err)
		require.Equal(t, pb.Deployment_ERROR, got.GetStatus())
		require.Equal(t, "clock failed", got.GetError())
	})

	t.Run("error over stopped", func(t *testing.T) {
		store := NewMemoryStore()
		msg := "already stopped"
		store.Put(&pb.Deployment{Id: "dep-1", Status: pb.Deployment_STOPPED, Error: &msg})

		err := store.UpdateStatus(ctx, "dep-1", pb.Deployment_ERROR, errors.New("late loop"))
		require.NoError(t, err)

		got, err := store.GetDeployment(ctx, "dep-1")
		require.NoError(t, err)
		require.Equal(t, pb.Deployment_STOPPED, got.GetStatus())
		require.Equal(t, "already stopped", got.GetError())
	})

	t.Run("non-error still writes", func(t *testing.T) {
		store := NewMemoryStore()
		msg := "already stopped"
		store.Put(&pb.Deployment{Id: "dep-1", Status: pb.Deployment_STOPPED, Error: &msg})

		err := store.UpdateStatus(ctx, "dep-1", pb.Deployment_STOPPED, errors.New("stopped note"))
		require.NoError(t, err)

		got, err := store.GetDeployment(ctx, "dep-1")
		require.NoError(t, err)
		require.Equal(t, pb.Deployment_STOPPED, got.GetStatus())
		require.Equal(t, "stopped note", got.GetError())
	})
}
