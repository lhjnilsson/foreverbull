package runner

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	commonpb "github.com/lhjnilsson/foreverbull/pkg/pb"
	financepb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/repository"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ErrDuplicateRun is a second tick for the same deployment session date.
var ErrDuplicateRun = errors.New("duplicate session run")

// Store is deployment + run persistence used by the in-process runner.
// Tests use MemoryStore (not SQL mocks). Production uses PostgresStore.
type Store interface {
	GetDeployment(ctx context.Context, id string) (*pb.Deployment, error)
	ListRunning(ctx context.Context) ([]*pb.Deployment, error)
	SetLastSessionDate(ctx context.Context, id string, date *commonpb.Date) error
	UpdateStatus(ctx context.Context, id string, status pb.Deployment_Status, err error) error
	CreateRun(ctx context.Context, deploymentID string, sessionDate *commonpb.Date, orders []*financepb.Order, skipped bool, runErr error) (*pb.DeploymentRun, error)
	ListRuns(ctx context.Context, strategyName, deploymentID string) ([]*pb.DeploymentRun, error)
}

// PostgresStore writes strategy_deployment / strategy_deployment_run.
type PostgresStore struct {
	Conn postgres.Query
}

func NewPostgresStore(conn postgres.Query) *PostgresStore {
	return &PostgresStore{Conn: conn}
}

var _ Store = (*PostgresStore)(nil)

func (s *PostgresStore) GetDeployment(ctx context.Context, id string) (*pb.Deployment, error) {
	d := repository.Deployment{Conn: s.Conn}
	return d.Get(ctx, id)
}

func (s *PostgresStore) ListRunning(ctx context.Context) ([]*pb.Deployment, error) {
	d := repository.Deployment{Conn: s.Conn}
	return d.ListRunning(ctx)
}

func (s *PostgresStore) SetLastSessionDate(ctx context.Context, id string, date *commonpb.Date) error {
	d := repository.Deployment{Conn: s.Conn}
	return d.SetLastSessionDate(ctx, id, date)
}

func (s *PostgresStore) UpdateStatus(ctx context.Context, id string, status pb.Deployment_Status, err error) error {
	d := repository.Deployment{Conn: s.Conn}
	return d.UpdateStatus(ctx, id, status, err)
}

func (s *PostgresStore) CreateRun(
	ctx context.Context,
	deploymentID string,
	sessionDate *commonpb.Date,
	orders []*financepb.Order,
	skipped bool,
	runErr error,
) (*pb.DeploymentRun, error) {
	runs := repository.DeploymentRun{Conn: s.Conn}
	run, err := runs.Create(ctx, deploymentID, sessionDate, orders, skipped, runErr)
	if isUniqueViolation(err) {
		return nil, ErrDuplicateRun
	}
	return run, err
}

func (s *PostgresStore) ListRuns(ctx context.Context, strategyName, deploymentID string) ([]*pb.DeploymentRun, error) {
	runs := repository.DeploymentRun{Conn: s.Conn}
	return runs.List(ctx, strategyName, deploymentID)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// MemoryStore is an in-process Store for runner unit tests.
type MemoryStore struct {
	mu          sync.Mutex
	deployments map[string]*pb.Deployment
	runs        []*pb.DeploymentRun
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{deployments: map[string]*pb.Deployment{}}
}

var _ Store = (*MemoryStore)(nil)

func (m *MemoryStore) Put(d *pb.Deployment) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deployments[d.Id] = cloneDeployment(d)
}

func (m *MemoryStore) GetDeployment(_ context.Context, id string) (*pb.Deployment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.deployments[id]
	if !ok {
		return nil, fmt.Errorf("deployment %q not found", id)
	}
	return cloneDeployment(d), nil
}

func (m *MemoryStore) ListRunning(_ context.Context) ([]*pb.Deployment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []*pb.Deployment{}
	for _, d := range m.deployments {
		if d.Status == pb.Deployment_RUNNING {
			out = append(out, cloneDeployment(d))
		}
	}
	return out, nil
}

func (m *MemoryStore) SetLastSessionDate(_ context.Context, id string, date *commonpb.Date) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.deployments[id]
	if !ok {
		return fmt.Errorf("deployment %q not found", id)
	}
	d.LastSessionDate = cloneDate(date)
	return nil
}

func (m *MemoryStore) UpdateStatus(_ context.Context, id string, status pb.Deployment_Status, err error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.deployments[id]
	if !ok {
		return fmt.Errorf("deployment %q not found", id)
	}
	d.Status = status
	if err != nil {
		msg := err.Error()
		d.Error = &msg
	} else {
		d.Error = nil
	}
	return nil
}

func (m *MemoryStore) CreateRun(
	_ context.Context,
	deploymentID string,
	sessionDate *commonpb.Date,
	orders []*financepb.Order,
	skipped bool,
	runErr error,
) (*pb.DeploymentRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, existing := range m.runs {
		if existing.DeploymentId == deploymentID && dateEqual(existing.SessionDate, sessionDate) {
			return nil, ErrDuplicateRun
		}
	}

	run := &pb.DeploymentRun{
		Id:           uuid.NewString(),
		DeploymentId: deploymentID,
		SessionDate:  cloneDate(sessionDate),
		StartedAt:    timestamppb.Now(),
		Orders:       cloneOrders(orders),
		Skipped:      skipped,
	}
	if runErr != nil {
		msg := runErr.Error()
		run.Error = &msg
	}
	m.runs = append(m.runs, run)
	return cloneRun(run), nil
}

func (m *MemoryStore) ListRuns(_ context.Context, _, deploymentID string) ([]*pb.DeploymentRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []*pb.DeploymentRun{}
	for _, r := range m.runs {
		if deploymentID == "" || r.DeploymentId == deploymentID {
			out = append(out, cloneRun(r))
		}
	}
	return out, nil
}

func cloneDeployment(d *pb.Deployment) *pb.Deployment {
	if d == nil {
		return nil
	}
	return proto.Clone(d).(*pb.Deployment)
}

func cloneRun(r *pb.DeploymentRun) *pb.DeploymentRun {
	if r == nil {
		return nil
	}
	return proto.Clone(r).(*pb.DeploymentRun)
}

func cloneDate(d *commonpb.Date) *commonpb.Date {
	if d == nil {
		return nil
	}
	return proto.Clone(d).(*commonpb.Date)
}

func cloneOrders(orders []*financepb.Order) []*financepb.Order {
	if orders == nil {
		return nil
	}
	out := make([]*financepb.Order, 0, len(orders))
	for _, o := range orders {
		out = append(out, proto.Clone(o).(*financepb.Order))
	}
	return out
}
