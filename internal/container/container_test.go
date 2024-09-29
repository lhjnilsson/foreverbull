package container

import (
	"context"
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/stretchr/testify/suite"
)

// Image: ziptest:latest
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
	e, err := NewEngine()
	test.Require().NoError(err)

	c, err := e.Start(context.TODO(), "ziptest:latest", "test")
	test.NoError(err)
	test.NotNil(c)

	status, err := c.GetStatus()
	test.NoError(err)
	test.Equal("running", status)

	for i := 0; i < 120; i++ {
		health, err := c.GetHealth()
		test.Require().NoError(err)
		if health == "healthy" {
			break
		}
		time.Sleep(time.Second / 4)
	}
	health, err := c.GetHealth()
	test.NoError(err)
	test.Equal("healthy", health)

	conn, err := c.GetConnectionString()
	test.NoError(err)
	test.NotEmpty(conn)
}
