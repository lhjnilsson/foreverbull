package worker_test

import (
	"testing"

	"github.com/lhjnilsson/foreverbull/pkg/service/worker"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestNamespaceGetSetFlush(t *testing.T) {
	t.Parallel()

	ns := worker.CreateNamespace([]string{"algo"})
	require.Nil(t, ns.Get("missing"))
	require.NotNil(t, ns.Get("algo"))
	require.Empty(t, ns.Get("algo").Fields)

	value, err := structpb.NewStruct(map[string]interface{}{"lookback": 20.0})
	require.NoError(t, err)
	require.Error(t, ns.Set("missing", value))
	require.NoError(t, ns.Set("algo", value))
	require.InDelta(t, 20.0, ns.Get("algo").Fields["lookback"].GetNumberValue(), 1e-9)

	ns.Flush()
	require.Empty(t, ns.Get("algo").Fields)
}

type ContainerTest struct {
	suite.Suite
}

func (test *ContainerTest) SetupTest() {
}

func (test *ContainerTest) TearDownTest() {
}

func TestNamespace(t *testing.T) {
	suite.Run(t, new(ContainerTest))
}
