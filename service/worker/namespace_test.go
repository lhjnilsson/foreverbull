package worker

import (
	"encoding/json"
	"testing"

	"github.com/lhjnilsson/foreverbull/service/entity"
	"github.com/stretchr/testify/suite"
)

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

func (test *ContainerTest) TestObjectContainer() {
	n := objectContainer[string]{
		items: map[string]string{},
	}

	err := n.Set(`{"key": "value"}`)
	test.NoError(err)

	value, isType := n.Get().(map[string]string)
	test.True(isType)
	test.Equal("value", value["key"])
}

func (test *ContainerTest) TestArrayContainer() {
	n := arrayContainer[string]{
		items: []string{},
	}

	err := n.Set(`["value"]`)
	test.NoError(err)

	value, isType := n.Get().([]string)
	test.True(isType)
	test.Equal("value", value[0])
}

func (test *ContainerTest) TestNamespace() {
	algoS := `{"namespace": {"rsi": {"type": "object", "value_type": "float64"}, "symbols": {"type": "array", "value_type": "string"}}}`
	algo := entity.Algorithm{}
	err := json.Unmarshal([]byte(algoS), &algo)
	test.NoError(err)

	n, err := CreateNamespace(&algo)
	test.NoError(err)

	err = n.Set("rsi", `{"key": 1.0}`)
	test.NoError(err)

	value, err := n.Get("rsi")
	test.NoError(err)
	v, isType := value.(map[string]float64)
	test.True(isType)
	test.Equal(1.0, v["key"])

	err = n.Set("symbols", `["AAPL"]`)
	test.NoError(err)

	value, err = n.Get("symbols")
	test.NoError(err)
	v2, isType := value.([]string)
	test.True(isType)
	test.Equal("AAPL", v2[0])
}
