package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lhjnilsson/foreverbull/pkg/service/api"
	"github.com/lhjnilsson/foreverbull/pkg/service/container"
	"github.com/rs/zerolog/log"
)

func GetImage(c *gin.Context) {
	var uri api.ImageURI
	if err := c.ShouldBindUri(&uri); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"msg": err})
		return
	}
	var name string
	if uri.Name[0] == '/' {
		name = uri.Name[1:]
	} else {
		name = uri.Name
	}

	images := c.MustGet(ImageDependency).(container.Image)
	image, err := images.Info(c, name)
	if err != nil {
		if strings.Contains(err.Error(), "No such image") {
			c.JSON(http.StatusNotFound, gin.H{"msg": err})
			return
		}
		fmt.Println("ERROR: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"msg": err})
		return
	}
	c.JSON(http.StatusOK, image)
}

func PullImage(c *gin.Context) {
	var uri api.ImageURI
	if err := c.ShouldBindUri(&uri); err != nil {
		log.Debug().Err(err).Msg("failed to bind uri")
		c.JSON(http.StatusBadRequest, gin.H{"msg": err})
		return
	}
	var name string
	if uri.Name[0] == '/' {
		name = uri.Name[1:]
	} else {
		name = uri.Name
	}

	images := c.MustGet(ImageDependency).(container.Image)
	image, err := images.Pull(c, name)
	if err != nil {
		log.Err(err).Msg("failed to pull image")
		c.JSON(http.StatusInternalServerError, gin.H{"msg": err})
		return
	}
	c.JSON(http.StatusCreated, image)
}
