package api

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/gin-gonic/gin"
	"github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/pkg/service/container"
	intContainer "github.com/lhjnilsson/foreverbull/pkg/service/internal/container"
	"github.com/stretchr/testify/suite"
)

type ImageTest struct {
	suite.Suite

	router *gin.Engine
	image  container.Image
}

func (test *ImageTest) SetupTest() {
	var err error
	test.image, err = intContainer.NewImageRegistry()
	test.Require().NoError(err)

	test.router = http.NewEngine()
	test.router.Use(
		func(ctx *gin.Context) {
			ctx.Set(ImageDependency, test.image)
			ctx.Next()
		},
	)
	test.router.GET("/images/*name", GetImage)
	test.router.POST("/images/*name", PullImage)
}

func TestImage(t *testing.T) {
	suite.Run(t, new(ImageTest))
}

func (test *ImageTest) TestGetAndPull() {
	// Delete image in case it exists and end with remove to cleanup
	client, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	test.Require().NoError(err)
	_, _ = client.ImageRemove(context.TODO(), "docker.io/library/python:3.12-alpine", image.RemoveOptions{})
	defer func() {
		_, _ = client.ImageRemove(context.TODO(), "docker.io/library/python:3.12-alpine", image.RemoveOptions{})
	}()

	test.Run("get", func() {
		req := httptest.NewRequest("GET", "/images/docker.io/library/python:3.12-alpine", nil)
		w := httptest.NewRecorder()
		test.router.ServeHTTP(w, req)
		test.Equal(404, w.Code)
	})
	test.Run("pull", func() {
		req := httptest.NewRequest("POST", "/images/docker.io/library/python:3.12-alpine", nil)
		w := httptest.NewRecorder()
		test.router.ServeHTTP(w, req)
		test.Equal(201, w.Code)
	})
	test.Run("get", func() {
		req := httptest.NewRequest("GET", "/images/docker.io/library/python:3.12-alpine", nil)
		w := httptest.NewRecorder()
		test.router.ServeHTTP(w, req)
		test.Equal(200, w.Code)
	})
}
