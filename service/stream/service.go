package stream

import (
	"github.com/google/uuid"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/service/entity"
)

type UpdateServiceStatusCommand struct {
	Name   string                   `json:"name"`
	Status entity.ServiceStatusType `json:"status"`
	Error  error                    `json:"error"`
}

type ServiceStartCommand struct {
	Name       string `json:"name"`
	InstanceID string `json:"instance_id"`
}

func NewUpdateServiceStatusCommand(name string, status entity.ServiceStatusType, err error) (stream.Message, error) {
	entity := &UpdateServiceStatusCommand{
		Name:   name,
		Status: status,
		Error:  err,
	}
	return stream.NewMessage("service", "service", "status", entity)
}

func NewInstanceID() string {
	return uuid.New().String()
}

func NewServiceStartCommand(serviceName, instanceID string) (stream.Message, error) {
	entity := &ServiceStartCommand{
		Name:       serviceName,
		InstanceID: instanceID,
	}
	return stream.NewMessage("service", "service", "start", entity)
}

func NewServiceInterviewOrchestration(serviceName string) (*stream.MessageOrchestration, error) {
	orchestration := stream.NewMessageOrchestration("service interview")

	instanceID := NewInstanceID()
	msg, err := NewServiceStartCommand(serviceName, instanceID)
	if err != nil {
		return nil, err
	}
	msg2, err := NewUpdateServiceStatusCommand(serviceName, entity.ServiceStatusInterview, nil)
	if err != nil {
		return nil, err
	}

	orchestration.AddStep("start service", []stream.Message{msg, msg2})
	msg, err = NewInstanceSanityCheckCommand([]string{instanceID})
	if err != nil {
		return nil, err
	}
	orchestration.AddStep("sanity check", []stream.Message{msg})
	msg, err = NewInstanceInterviewCommand(instanceID)
	if err != nil {
		return nil, err
	}
	orchestration.AddStep("interview", []stream.Message{msg})
	msg, err = NewInstanceStopCommand(instanceID)
	if err != nil {
		return nil, err
	}
	msg2, err = NewUpdateServiceStatusCommand(serviceName, entity.ServiceStatusReady, nil)
	if err != nil {
		return nil, err
	}
	orchestration.AddStep("stop service", []stream.Message{msg, msg2})

	msg, err = NewInstanceStopCommand(instanceID)
	if err != nil {
		return nil, err
	}
	msg2, err = NewUpdateServiceStatusCommand(serviceName, entity.ServiceStatusError, nil)
	if err != nil {
		return nil, err
	}
	orchestration.SettFallback([]stream.Message{msg, msg2})

	return orchestration, nil
}
