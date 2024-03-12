package api

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/service/internal/repository"
	"github.com/stretchr/testify/assert"
)

func AddService(t *testing.T, conn *pgxpool.Pool, image string) string {
	repository := repository.Service{Conn: conn}
	service, err := repository.Create(context.Background(), image)
	assert.Nil(t, err)
	return service.Image
}

func AddInstance(t *testing.T, conn *pgxpool.Pool, image string) string {
	repository := repository.Instance{Conn: conn}
	instance, err := repository.Create(context.Background(), uuid.New().String(), image)
	assert.Nil(t, err)
	return instance.ID
}
