package backtest

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/pb"
	backtest_pb "github.com/lhjnilsson/foreverbull/internal/pb/backtest"
	service_pb "github.com/lhjnilsson/foreverbull/internal/pb/service"
	"github.com/lhjnilsson/foreverbull/internal/socket"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/entity"
	service "github.com/lhjnilsson/foreverbull/pkg/service/entity"
	"github.com/lhjnilsson/foreverbull/pkg/service/worker"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/proto"
)

type manualSession struct {
	session session `json:"-"`

	backtest engine.Engine `json:"-"`
	workers  worker.Pool   `json:"-"`

	Socket socket.Replier `json:"-"`

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
		err = ms.Socket.Close()
		if err != nil {
			log.Err(err).Msg("failed to close socket")
		}
	}()
	for {
		request := service_pb.Request{}
		msg, err := ms.Socket.Recieve(&request, socket.WithReadTimeout(time.Second), socket.WithSendTimeout(time.Second))
		if err != nil {
			if err == socket.ReadTimeout {
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
		rsp := service_pb.Response{Task: request.Task}
		switch request.Task {
		case "new_execution":
			if ms.execution != nil {
				err = errors.New("execution already exists")
				break
			}
			execution_req := backtest_pb.NewExecutionRequest{}
			err = proto.Unmarshal(request.Data, &execution_req)
			if err != nil {
				err = errors.New("failed to unmarshal data")
				break
			}
			ms.executionAlgo = &service.Algorithm{
				FilePath:   execution_req.Algorithm.FilePath,
				Namespaces: execution_req.Algorithm.Namespaces,
			}
			for _, function := range execution_req.Algorithm.Functions {
				function_def := service.AlgorithmFunction{
					Name:              function.Name,
					ParallelExecution: function.ParallelExecution,
					RunFirst:          function.RunFirst,
					RunLast:           function.RunLast,
				}
				for _, parameter := range function.Parameters {
					function_def.Parameters = append(function_def.Parameters, service.FunctionParameter{
						Key:     parameter.Key,
						Default: parameter.DefaultValue,
						Type:    parameter.ValueType,
					})
				}
				ms.executionAlgo.Functions = append(ms.executionAlgo.Functions, function_def)
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
			execution_rsp := backtest_pb.NewExecutionResponse{
				Id:        ms.executionEntity.ID,
				StartDate: pb.TimeToProtoTimestamp(ms.session.backtest.Start),
				EndDate:   pb.TimeToProtoTimestamp(ms.session.backtest.End),
				Symbols:   ms.session.backtest.Symbols,
			}
			rsp.Data, err = proto.Marshal(&execution_rsp)
			if err != nil {
				err = fmt.Errorf("failed to marshal data: %w", err)
				break
			}
		case "configure_execution":
			functions, confErr := ms.executionAlgo.Configure()
			if confErr != nil {
				err = fmt.Errorf("failed to get configure function: %w", err)
				break
			}
			brokerPort := ms.workers.GetPort()
			namespacePort := ms.workers.GetNamespacePort()
			config_request := backtest_pb.ConfigureExecutionRequest{}
			err = proto.Unmarshal(request.Data, &config_request)
			if err != nil {
				err = errors.New("failed to unmarshal data")
				break
			}
			ms.executionEntity.Start = config_request.StartDate.AsTime()
			ms.executionEntity.End = config_request.EndDate.AsTime()
			ms.executionEntity.Symbols = config_request.Symbols
			ms.executionEntity.Benchmark = config_request.Benchmark

			config_response := service_pb.ConfigureExecutionRequest{
				BrokerPort:    int32(brokerPort),
				NamespacePort: int32(namespacePort),
				DatabaseURL:   environment.GetPostgresURL(),
			}
			for name, function := range functions {
				f_def := service_pb.ConfigureExecutionRequest_Function{
					Name: name,
				}
				for key, value := range function.Parameters {
					f_def.Parameters = append(f_def.Parameters, &service_pb.ConfigureExecutionRequest_FunctionParameter{
						Key:   key,
						Value: value,
					})
				}
				config_response.Functions = append(config_response.Functions, &f_def)
			}
			rsp.Data, err = proto.Marshal(&config_response)
			if err != nil {
				err = fmt.Errorf("failed to marshal data: %w", err)
				break
			}
		case "run_execution":
			err = ms.execution.Configure(context.Background(), ms.executionEntity)
			if err != nil {
				return fmt.Errorf("failed to configure execution: %w", err)
			}
			go func() {
				err = ms.execution.Run(context.TODO(), ms.executionEntity)
				if err != nil {
					log.Err(err).Msg("failed to run execution")
					return
				}
				periods, err := ms.execution.StoreDataFrameAndGetPeriods(context.Background(), ms.executionEntity.ID)
				if err != nil {
					log.Err(err).Msg("failed to get periods")
					return
				}
				for _, period := range *periods {
					err = ms.session.periods.Store(context.Background(), ms.executionEntity.ID, &period)
					if err != nil {
						log.Err(err).Msg("failed to store period")
						return
					}
				}
				ms.execution = nil
				ms.executionEntity = nil
			}()
		case "current_portfolio":
			if ms.execution == nil {
				rsp.Data = nil
			} else {
				portfolio := ms.execution.CurrentPortfolio()
				data, err := proto.Marshal(portfolio)
				if err != nil {
					return fmt.Errorf("failed to marshal data: %w", err)
				}
				rsp.Data = data
			}
		case "stop":
			log.Info().Str("session", ms.session.session.ID).Msg("stopping session")
			err = msg.Reply(&rsp)
			if err != nil {
				return fmt.Errorf("failed to reply: %w", err)
			}
			return nil
		default:
			err = errors.New("unknown task")
		}
		if err != nil {
			errStr := err.Error()
			rsp.Error = &errStr
		}
		err = msg.Reply(&rsp)
		if err != nil {
			return fmt.Errorf("failed to reply: %w", err)
		}
	}
}

func (ms *manualSession) Stop(ctx context.Context) error {
	err := ms.workers.Close()
	if err != nil {
		log.Err(err).Msg("failed to close workers")
		return err
	}
	err = ms.Socket.Close()
	if err != nil && err != socket.Closed {
		log.Err(err).Msg("failed to close socket")
		return err
	}
	return nil
}
