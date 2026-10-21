package backtest

import (
	"context"
	"fmt"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
)

// Store is a narrow facade over backtest universes and sessions so other
// domains can start the existing session/execution path without importing
// pkg/backtest/internal.
type Store interface {
	GetBacktest(ctx context.Context, name string) (*pb.Backtest, error)
	CreateSession(ctx context.Context, backtestName string) (*pb.Session, error)
	GetSession(ctx context.Context, id string) (*pb.Session, error)
}

type store struct {
	backtests repository.Backtest
	sessions  repository.Session
}

func NewStore(conn postgres.Query) Store {
	return &store{
		backtests: repository.Backtest{Conn: conn},
		sessions:  repository.Session{Conn: conn},
	}
}

var _ Store = (*store)(nil)

func (s *store) GetBacktest(ctx context.Context, name string) (*pb.Backtest, error) {
	if environment.GetAuthEnabled() {
		return nil, fmt.Errorf("get backtest by name is refused while AUTH_ENABLED=true")
	}
	b, err := s.backtests.Get(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("get backtest: %w", err)
	}
	return b, nil
}

func (s *store) CreateSession(ctx context.Context, backtestName string) (*pb.Session, error) {
	if environment.GetAuthEnabled() {
		return nil, fmt.Errorf("create session by name is refused while AUTH_ENABLED=true")
	}
	session, err := s.sessions.Create(ctx, backtestName)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return session, nil
}

func (s *store) GetSession(ctx context.Context, id string) (*pb.Session, error) {
	session, err := s.sessions.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	return session, nil
}

// GetBacktestByID loads one backtest after the caller resolved its id.
func (s *store) GetBacktestByID(ctx context.Context, id string) (*pb.Backtest, error) {
	b, err := s.backtests.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get backtest: %w", err)
	}
	return b, nil
}

// CreateSessionFor inserts a session for a backtest id.
func (s *store) CreateSessionFor(ctx context.Context, backtestID string) (*pb.Session, error) {
	session, err := s.sessions.CreateForBacktest(ctx, backtestID)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return session, nil
}
