package container

import (
	"context"
	"testing"

	dockerImage "github.com/docker/docker/api/types/image"
	"github.com/stretchr/testify/suite"
)

type ImageTest struct {
	suite.Suite

	image
}

func (test *ImageTest) SetupTest() {
	i, err := NewImageRegistry()
	test.Require().NoError(err)
	image, ok := i.(*image)
	test.Require().True(ok)
	test.image = *image
}

func (test *ImageTest) TearDownTest() {
}

func TestImage(t *testing.T) {
	suite.Run(t, new(ImageTest))
}

func (test *ImageTest) TestInfoAndPull() {
	// Delete image in case it exists and end with remove to cleanup
	_, _ = test.image.client.ImageRemove(context.TODO(), "docker.io/library/python:3.12-alpine", dockerImage.RemoveOptions{})
	defer func() {
		_, _ = test.image.client.ImageRemove(context.TODO(), "docker.io/library/python:3.12-alpine", dockerImage.RemoveOptions{})
	}()
	test.Run("info, not stored", func() {
		_, err := test.image.Info(context.TODO(), "docker.io/library/python:3.12-alpine")
		test.Require().Error(err)
		test.Require().Contains(err.Error(), "No such image")
	})
	test.Run("pull", func() {
		_, err := test.image.Pull(context.TODO(), "docker.io/library/python:3.12-alpine")
		test.Require().NoError(err)
	})
	test.Run("info, stored", func() {
		i, err := test.image.Info(context.TODO(), "docker.io/library/python:3.12-alpine")
		test.Require().NoError(err)
		test.Require().NotNil(i)
	})
}
