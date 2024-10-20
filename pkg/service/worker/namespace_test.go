package worker_test

import (
	"testing"

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
