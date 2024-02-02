package backtest

import (
	"context"
	"errors"
	"fmt"

	"github.com/lhjnilsson/foreverbull/backtest/entity"
	"github.com/lhjnilsson/foreverbull/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/service/backtest/engine"
	service "github.com/lhjnilsson/foreverbull/service/entity"
	"github.com/lhjnilsson/foreverbull/service/message"
	"github.com/lhjnilsson/foreverbull/service/socket"
	"github.com/lhjnilsson/foreverbull/service/worker"
	"go.nanomsg.org/mangos/v3"
	"go.uber.org/zap"
)

type Session interface {
	GetSocket() *socket.Socket
	Run(chan<- bool, <-chan bool) error
	RunExecution(ctx context.Context, execution *entity.Execution) error
	Stop(ctx context.Context) error
}

type session struct {
	log *zap.Logger

	backtest         *entity.Backtest `json:"-"`
	session          *entity.Session
	backtestInstance *service.Instance     `json:"-"`
	executions       *repository.Execution `json:"-"`
	periods          *repository.Period    `json:"-"`
	orders           *repository.Order     `json:"-"`
	portfolio        *repository.Portfolio `json:"-"`

	engine  engine.Engine `json:"-"`
	workers worker.Pool   `json:"-"`

	Socket          socket.Socket        `json:"-"`
	socket          socket.ContextSocket `json:"-"`
	execution       Execution            `json:"-"`
	executionEntity *entity.Execution    `json:"-"`
}

func NewSession(ctx context.Context, log *zap.Logger,
	storedBacktest *entity.Backtest, storedSession *entity.Session, backtestInstance *service.Instance,
	executions *repository.Execution, periods *repository.Period, orders *repository.Order, portfolio *repository.Portfolio,
	workers ...*service.Instance) (Session, error) {
	engine, err := engine.NewZiplineEngine(ctx, backtestInstance)
	if err != nil {
		return nil, fmt.Errorf("error creating zipline engine: %w", err)
	}
	workerPool, err := worker.NewPool(ctx, workers...)
	if err != nil {
		return nil, err
	}

	s := session{
		log:              log,
		backtest:         storedBacktest,
		session:          storedSession,
		backtestInstance: backtestInstance,
		executions:       executions,
		periods:          periods,
		orders:           orders,
		portfolio:        portfolio,

		engine:  engine,
		workers: workerPool,
	}
	s.Socket = socket.Socket{Type: socket.Replier, Host: "0.0.0.0", Port: 0, Listen: true, Dial: false}
	s.socket, err = socket.GetContextSocket(ctx, &s.Socket)
	if err != nil {
		return nil, err
	}
	return &s, engine.DownloadIngestion(ctx, storedBacktest.Name)
}

func (e *session) GetSocket() *socket.Socket {
	return &e.Socket
}

func (e *session) RunExecution(ctx context.Context, execution *entity.Execution) error {
	exec := NewExecution(e.engine, e.workers)

	workerCfg := worker.Configuration{
		Execution: execution.ID,
		Port:      e.workers.SocketConfig().Port,
		Database:  environment.GetPostgresURL(),
	}

	tz := "UTC"
	backtestCfg := engine.BacktestConfig{
		Calendar:  &execution.Calendar,
		Start:     &execution.Start,
		End:       &execution.End,
		Timezone:  &tz,
		Benchmark: execution.Benchmark,
		Symbols:   &execution.Symbols,
	}

	err := exec.Configure(ctx, &workerCfg, &backtestCfg)
	if err != nil {
		return fmt.Errorf("failed to configure execution: %w", err)
	}

	err = e.executions.UpdateParameters(ctx, execution.ID, &execution.Parameters)
	if err != nil {
		return fmt.Errorf("failed to update execution parameters: %w", err)
	}
	e.log.Info("backtestCFG", zap.Any("backtestCfg", backtestCfg))
	err = e.executions.UpdateSimulationDetails(ctx, execution.ID, *backtestCfg.Calendar, *backtestCfg.Start, *backtestCfg.End, "FIXME", *backtestCfg.Symbols)
	if err != nil {
		return fmt.Errorf("failed to update execution simulation details: %w", err)
	}

	events := make(chan chan entity.ExecutionPeriod)
	go exec.Run(context.TODO(), execution.ID, events)
	for event := range events {
		status := <-event
		if status.Error != nil {
			e.log.Error("Failed to run execution", zap.Error(status.Error))
		} else {
			for _, order := range status.Period.NewOrders {
				err = e.orders.Store(context.Background(), execution.ID, &order)
				if err != nil {
					e.log.Error("Failed to create order", zap.Error(err))
				}
			}
			err = e.portfolio.Store(context.Background(), execution.ID, status.Period.Timestamp, &status.Period.Portfolio)
			if err != nil {
				e.log.Error("Failed to create position", zap.Error(err))
			}
		}
		close(event)
	}
	periods, err := exec.StoreDataFrameAndGetPeriods(context.Background(), execution.ID)
	if err != nil {
		e.log.Error("Failed to store data frame and get periods", zap.Error(err))
		return err
	}
	for _, period := range *periods {
		err = e.periods.Store(context.Background(), execution.ID, &period)
		if err != nil {
			e.log.Error("Failed to store period", zap.Error(err))
			return err
		}
	}
	return nil
}

func (e *session) Run(activity chan<- bool, stop <-chan bool) error {
	defer e.workers.Stop(context.TODO())
	defer e.engine.Stop(context.TODO())
	defer e.socket.Close()
	for {
		socket, err := e.socket.Get()
		if err != nil {
			e.log.Error("Failed to get socket", zap.Error(err))
			return err
		}
		defer socket.Close()
		var rsp message.Response
		byteMsg, err := socket.Read()
		if err != nil {
			if err == mangos.ErrRecvTimeout {
				select {
				case <-stop:
					return fmt.Errorf("received stop signal")
				default:
					continue
				}
			}
			e.log.Error("Failed to read message", zap.Error(err))
			return err
		}
		activity <- true
		req := message.Request{}
		if err = req.Decode(byteMsg); err != nil {
			e.log.Error("Failed to decode message", zap.Error(err))
			continue
		}
		rsp = message.Response{Task: req.Task}
		e.log.Debug("Received request", zap.String("task", req.Task), zap.Any("data", req.Data))
		switch req.Task {
		case "new_execution":
			if e.execution != nil {
				e.log.Error("execution already exists")
				err = errors.New("execution already exists")
				break
			}
			e.executionEntity, err = e.executions.Create(context.Background(), e.session.ID, e.backtest.Calendar, e.backtest.Start, e.backtest.End, e.backtest.Symbols, e.backtest.Benchmark)
			if err != nil {
				e.log.Error("Failed to create execution", zap.Error(err))
				err = errors.New("failed to create execution")
				break
			}
			e.executionEntity.Database = environment.GetPostgresURL()

			e.executionEntity.Port = &e.workers.SocketConfig().Port

			e.execution = NewExecution(e.engine, e.workers)
			rsp = message.Response{Task: req.Task, Data: e.executionEntity}
		case "run_execution":
			events := make(chan chan entity.ExecutionPeriod)
			tz := "UTC"
			backtestCfg := engine.BacktestConfig{
				Calendar:  &e.executionEntity.Calendar,
				Start:     &e.executionEntity.Start,
				End:       &e.executionEntity.End,
				Timezone:  &tz,
				Benchmark: e.executionEntity.Benchmark,
				Symbols:   &e.executionEntity.Symbols,
			}
			err = e.execution.Configure(context.Background(), nil, &backtestCfg)
			if err != nil {
				e.log.Error("Failed to configure execution", zap.Error(err))
				return err
			}
			go e.execution.Run(context.TODO(), e.executionEntity.ID, events)
			for event := range events {
				activity <- true
				status := <-event
				if status.Error != nil {
					e.log.Error("Failed to run execution", zap.Error(status.Error))
					rsp.Error = status.Error.Error()
				} else {
					for _, order := range status.Period.NewOrders {
						err = e.orders.Store(context.Background(), e.executionEntity.ID, &order)
						if err != nil {
							e.log.Error("Failed to create order", zap.Error(err))
						}
					}
					err = e.portfolio.Store(context.Background(), e.executionEntity.ID, status.Period.Timestamp, &status.Period.Portfolio)
					if err != nil {
						e.log.Error("Failed to create position", zap.Error(err))
					}
				}
				close(event)
			}
			if rsp.Error == "" {
				periods, err := e.execution.StoreDataFrameAndGetPeriods(context.Background(), e.executionEntity.ID)
				if err != nil {
					e.log.Error("Failed to store data frame and get periods", zap.Error(err))
					return err
				}
				for _, period := range *periods {
					err = e.periods.Store(context.Background(), e.executionEntity.ID, &period)
					if err != nil {
						e.log.Error("Failed to store period", zap.Error(err))
						return err
					}
				}
			}
			e.execution = nil
			e.executionEntity = nil
		case "stop":
			byteMsg, err = rsp.Encode()
			if err != nil {
				e.log.Error("Failed to encode message", zap.Error(err))
				return err
			}
			if err = socket.Write(byteMsg); err != nil {
				e.log.Error("Failed to write message", zap.Error(err))
				return err
			}
			return nil
		default:
			e.log.Error("Unknown task", zap.String("task", req.Task))
			err = errors.New("unknown task")
		}
		if err != nil {
			rsp.Error = err.Error()
		}
		byteMsg, err = rsp.Encode()
		if err != nil {
			e.log.Error("Failed to encode message", zap.Error(err))
			return err
		}
		if err = socket.Write(byteMsg); err != nil {
			e.log.Error("Failed to write message", zap.Error(err))
			return err
		}
	}
}

func (e *session) Stop(ctx context.Context) error {
	if err := e.workers.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop workers: %w", err)
	}
	return e.socket.Close()
}
