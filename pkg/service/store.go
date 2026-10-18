package service

import (
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/pkg/service/internal/repository"
)

// NewStore returns the service table behind GET and POST /api/services.
// The image is the primary key. Callers outside this module use it instead of
// the internal repository.
func NewStore(conn postgres.Query) ServiceStore {
	return &repository.Service{Conn: conn}
}

// IsNotFound reports a missing image. repository.Service.Get uses this text,
// not pgx.ErrNoRows.
func IsNotFound(err error) bool {
	return err != nil && err.Error() == "service not found"
}
