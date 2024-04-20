package helper

import (
	"io"
	"os"
	"path"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func GetAsset(t *testing.T, fileName string) ([]byte, error) {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)

	f, err := os.Open(path.Dir(filename) + "/assets/" + fileName)
	require.NoError(t, err)
	defer f.Close()
	return io.ReadAll(f)
}
