package container

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	"github.com/stretchr/testify/suite"
)

type ContainerTest struct {
	suite.Suite

	container serviceContainer
}

func (test *ContainerTest) SetupTest() {
	c, err := New()
	test.Require().NoError(err)
	container, ok := c.(*serviceContainer)
	test.Require().True(ok)
	test.container = *container

	// Postgres, just to also create network
	helper.SetupEnvironment(test.T(), &helper.Containers{
		Postgres: true,
	})
}

func (test *ContainerTest) TearDownTest() {
}

func TestContainer(t *testing.T) {
	suite.Run(t, new(ContainerTest))
}

func (test *ContainerTest) TestHasImage() {
	has, err := test.container.hasImage(context.TODO(), helper.PostgresImage)
	test.Require().NoError(err)
	test.Require().True(has)
}

func (test *ContainerTest) TestPull() {
	// remove python image if it exists, and pull
	image := "docker.io/library/python:3.11-bookworm"

	client, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	test.Require().NoError(err)
	_, err = client.ImageRemove(context.TODO(), image, types.ImageRemoveOptions{})
	if err != nil {
		test.Require().Contains(err.Error(), "No such image")
	}

	err = test.container.Pull(context.TODO(), image)
	test.Require().NoError(err)
}

func (test *ContainerTest) TestInfo() {
	info, err := test.container.Info(context.TODO(), helper.PostgresImage)
	test.Require().NoError(err)
	test.Require().NotEmpty(info.ID)
}

func (test *ContainerTest) TestStartSaveStop() {
	// These subtests are ment to be run in order
	image := "docker.io/library/python:3.11-bookworm"
	var containerID string
	var err error
	newImageName := uuid.New().String()

	test.T().Cleanup(func() {
		client, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		test.Require().NoError(err)
		err = client.ContainerRemove(context.TODO(), containerID, container.RemoveOptions{Force: true})
		if err != nil {
			test.Contains(err.Error(), "No such container")
		}
		_, err = client.ImageRemove(context.TODO(), newImageName, types.ImageRemoveOptions{})
		test.Require().NoError(err)
	})

	test.Run("Start", func() {
		containerID, err = test.container.Start(context.TODO(), "test", image, "test", nil)
		test.Require().NoError(err)
		test.Require().NotEmpty(containerID)
	})
	test.Run("Save", func() {
		err = test.container.SaveImage(context.TODO(), containerID, newImageName)
		test.Require().NoError(err)
	})
	test.Run("Stop", func() {
		err = test.container.Stop(context.TODO(), containerID, true)
		test.Require().NoError(err)
	})
}

func (test *ContainerTest) TestStopAll() {
	// Start a container
	image := "docker.io/library/python:3.11-bookworm"
	containerID, err := test.container.Start(context.TODO(), "test", image, "test", nil)
	test.Require().NoError(err)
	test.Require().NotEmpty(containerID)

	// Stop all containers
	err = test.container.StopAll(context.TODO(), true)
	test.Require().NoError(err)

}
