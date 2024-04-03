package command

import (
	"context"
	"fmt"

	"github.com/lhjnilsson/foreverbull/finance/supplier"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/strategy/internal/repository"
	"github.com/lhjnilsson/foreverbull/strategy/internal/stream/dependency"
	ss "github.com/lhjnilsson/foreverbull/strategy/stream"
)

func RunExecution(ctx context.Context, message stream.Message) error {
	db := message.MustGet(stream.DBDep).(postgres.Query)

	command := ss.ExecutionRunCommand{}
	err := message.ParsePayload(&command)
	if err != nil {
		return fmt.Errorf("error unmarshalling ExecutionRun payload: %w", err)
	}

	executions := repository.Execution{Conn: db}
	_, err = executions.Get(ctx, command.ExecutionID)
	if err != nil {
		return fmt.Errorf("error getting execution: %w", err)
	}

	trading := message.MustGet(dependency.Trading).(supplier.Trading)
	portfolio, err := trading.GetPortfolio()
	if err != nil {
		return fmt.Errorf("error getting portfolio: %w", err)
	}
	err = executions.SetStartPortfolio(ctx, command.ExecutionID, portfolio)
	if err != nil {
		return fmt.Errorf("error setting start portfolio: %w", err)
	}
	executionRunner, err := message.Call(ctx, dependency.ExecutionRunner)
	if err != nil {
		return fmt.Errorf("error getting execution runner: %w", err)
	}
	ex := executionRunner.(dependency.Execution)
	err = ex.Configure(ctx)
	if err != nil {
		return fmt.Errorf("error configuring execution runner: %w", err)
	}
	orders, err := ex.Run(ctx, portfolio)
	if err != nil {
		return fmt.Errorf("error running execution: %w", err)
	}
	err = ex.Stop(ctx)
	if err != nil {
		return fmt.Errorf("error stopping execution: %w", err)
	}
	err = executions.SetPlacedOrders(ctx, command.ExecutionID, orders)
	if err != nil {
		return fmt.Errorf("error setting placed orders: %w", err)
	}

	return nil
}

func UpdateExecutionStatus(ctx context.Context, message stream.Message) error {
	db := message.MustGet(stream.DBDep).(postgres.Query)

	command := ss.UpdateExecutionStatusCommand{}
	err := message.ParsePayload(&command)
	if err != nil {
		return fmt.Errorf("error unmarshalling UpdateExecutionStatus payload: %w", err)
	}

	executions := repository.Execution{Conn: db}
	err = executions.UpdateStatus(ctx, command.ExecutionID, command.Status, command.Error)
	if err != nil {
		return fmt.Errorf("error updating execution status: %w", err)
	}

	return nil
}
