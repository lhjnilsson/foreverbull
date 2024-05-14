package backtest

import (
	"context"
	"errors"
	"fmt"

	"github.com/lhjnilsson/foreverbull/backtest/entity"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/service/backtest"
	service "github.com/lhjnilsson/foreverbull/service/entity"
	"github.com/lhjnilsson/foreverbull/service/message"
	"github.com/lhjnilsson/foreverbull/service/socket"
	"github.com/lhjnilsson/foreverbull/service/worker"
	"github.com/rs/zerolog/log"
	"go.nanomsg.org/mangos/v3"
)

type manualSession struct {
	session session `json:"-"`

	backtest backtest.Backtest `json:"-"`
	workers  worker.Pool       `json:"-"`

	Socket socket.Socket        `json:"-"`
	socket socket.ContextSocket `json:"-"`

	execution       Execution          `json:"-"`
	executionEntity *entity.Execution  `json:"-"`
	executionAlgo   *service.Algorithm `json:"-"`
}

func (ms *manualSession) Run(activity chan<- bool, stop <-chan bool) error {
	defer func() {
		err := ms.backtest.Stop(context.TODO())
		if err != nil {
			log.Err(err).Msg("failed to stop engine")
		}
		err = ms.socket.Close()
		if err != nil {
			log.Err(err).Msg("failed to close socket")
		}
	}()
	for {
		socket, err := ms.socket.Get()
		if err != nil {
			return fmt.Errorf("failed to get socket: %w", err)
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
			return err
		}
		activity <- true
		req := message.Request{}
		if err = req.Decode(byteMsg); err != nil {
			continue
		}
		rsp = message.Response{Task: req.Task}
		switch req.Task {
		case "new_execution":
			if ms.execution != nil {
				err = errors.New("execution already exists")
				break
			}
			ms.executionAlgo = &service.Algorithm{}
			err = req.DecodeData(ms.executionAlgo)
			if err != nil {
				err = errors.New("failed to decode data")
				break
			}
			err = ms.workers.SetAlgorithm(ms.executionAlgo)
			if err != nil {
				err = fmt.Errorf("failed to set algorithm: %w", err)
				break
			}
			ms.executionEntity, err = ms.session.executions.Create(context.Background(), ms.session.session.ID,
				ms.session.backtest.Calendar, ms.session.backtest.Start, ms.session.backtest.End, ms.session.backtest.Symbols, ms.session.backtest.Benchmark)
			if err != nil {
				err = fmt.Errorf("failed to create execution: %w", err)
				break
			}
			ms.execution = NewExecution(ms.backtest, ms.workers)
			rsp.Data = ms.executionEntity
		case "configure_execution":
			functions, confErr := ms.executionAlgo.Configure()
			if confErr != nil {
				err = fmt.Errorf("failed to get configure function: %w", err)
				break
			}
			brokerPort := ms.workers.GetPort()
			namespacePort := ms.workers.GetNamespacePort()
			instance := service.Instance{
				BrokerPort:    &brokerPort,
				NamespacePort: &namespacePort,
				DatabaseURL: func() *string {
					url := environment.GetPostgresURL()
					return &url
				}(),
				Functions: &functions,
			}
			rsp.Data = &instance
		case "run_execution":
			backtestCfg := backtest.BacktestConfig{
				Calendar: &ms.executionEntity.Calendar,
				Start:    &ms.executionEntity.Start,
				End:      &ms.executionEntity.End,
				Timezone: func() *string {
					tz := "UTC"
					return &tz
				}(),
				Benchmark: ms.executionEntity.Benchmark,
				Symbols:   &ms.executionEntity.Symbols,
			}
			err = ms.execution.Configure(context.Background(), &backtestCfg)
			if err != nil {
				return fmt.Errorf("failed to configure execution: %w", err)
			}
			err = ms.execution.Run(context.TODO(), ms.executionEntity)
			if err != nil {
				return fmt.Errorf("failed to run execution: %w", err)
			}
			if rsp.Error == "" {
				periods, err := ms.execution.StoreDataFrameAndGetPeriods(context.Background(), ms.executionEntity.ID)
				if err != nil {
					return fmt.Errorf("failed to store data frame and get periods: %w", err)
				}
				for _, period := range *periods {
					err = ms.session.periods.Store(context.Background(), ms.executionEntity.ID, &period)
					if err != nil {
						return fmt.Errorf("failed to store period: %w", err)
					}
				}
			}
			ms.execution = nil
			ms.executionEntity = nil
		case "stop":
			byteMsg, err = rsp.Encode()
			if err != nil {
				return fmt.Errorf("failed to encode message: %w", err)
			}
			if err = socket.Write(byteMsg); err != nil {
				return fmt.Errorf("failed to write message: %w", err)
			}
			return nil
		default:
			err = errors.New("unknown task")
		}
		if err != nil {
			rsp.Error = err.Error()
		}
		byteMsg, err = rsp.Encode()
		if err != nil {
			return fmt.Errorf("failed to encode message: %w", err)
		}
		if err = socket.Write(byteMsg); err != nil {
			return fmt.Errorf("failed to write message: %w", err)
		}
	}
}

func (ms *manualSession) Stop(ctx context.Context) error {
	return ms.socket.Close()
}
