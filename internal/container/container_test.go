package container

import (
	"context"
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/stretchr/testify/suite"
)

// Image: ziptest:latest.
type EngineTest struct {
	suite.Suite
}

func TestEngine(t *testing.T) {
	suite.Run(t, new(EngineTest))
}

func (test *EngineTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{})
}

func (test *EngineTest) NoTestStart() {
	engine, err := NewEngine()
	test.Require().NoError(err)

	container, err := engine.Start(context.TODO(), "ziptest:latest", "test")
	test.Require().NoError(err)
	test.NotNil(container)

	status, err := container.GetStatus()
	test.Require().NoError(err)
	test.Equal("running", status)

	for _ = range 120 {
		health, err := container.GetHealth()
		test.Require().NoError(err)

		if health == "healthy" {
			break
		}

		time.Sleep(time.Second / 4)
	}

	health, err := container.GetHealth()
	test.Require().NoError(err)
	test.Equal("healthy", health)

	conn, err := container.GetConnectionString()
	test.Require().NoError(err)
	test.NotEmpty(conn)
}
