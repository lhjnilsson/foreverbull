package stream

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/service/pb"
)

type UpdateServiceStatusCommand struct {
	Image  string
	Status pb.Service_Status_Status
	Error  error
}

type ServiceStartCommand struct {
	Image      string
	InstanceID string
}

func NewUpdateServiceStatusCommand(image string, status pb.Service_Status_Status, err error) (stream.Message, error) {
	entity := &UpdateServiceStatusCommand{
		Image:  image,
		Status: status,
		Error:  err,
	}

	msg, err := stream.NewMessage("service", "service", "status", entity)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %v", err)
	}

	return msg, nil
}

func NewInstanceID() string {
	return uuid.New().String()
}

func NewServiceStartCommand(image, instanceID string) (stream.Message, error) {
	entity := &ServiceStartCommand{
		Image:      image,
		InstanceID: instanceID,
	}

	msg, err := stream.NewMessage("service", "service", "start", entity)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %v", err)
	}

	return msg, nil
}

func NewServiceInterviewOrchestration(image string) (*stream.MessageOrchestration, error) {
	orchestration := stream.NewMessageOrchestration("service interview")

	instanceID := NewInstanceID()

	msg, err := NewServiceStartCommand(image, instanceID)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %v", err)
	}

	msg2, err := NewUpdateServiceStatusCommand(image, pb.Service_Status_INTERVIEW, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %v", err)
	}

	orchestration.AddStep("start service", []stream.Message{msg, msg2})

	msg, err = NewInstanceSanityCheckCommand([]string{instanceID})
	if err != nil {
		return nil, fmt.Errorf("error creating message: %v", err)
	}

	orchestration.AddStep("sanity check", []stream.Message{msg})

	msg, err = NewInstanceInterviewCommand(instanceID)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %v", err)
	}

	orchestration.AddStep("interview", []stream.Message{msg})

	msg, err = NewInstanceStopCommand(instanceID)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %v", err)
	}

	msg2, err = NewUpdateServiceStatusCommand(image, pb.Service_Status_READY, nil)
	if err != nil {
		return nil, err
	}

	orchestration.AddStep("stop service", []stream.Message{msg, msg2})

	msg, err = NewInstanceStopCommand(instanceID)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %v", err)
	}

	msg2, err = NewUpdateServiceStatusCommand(image, pb.Service_Status_ERROR, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %v", err)
	}

	orchestration.SettFallback([]stream.Message{msg, msg2})

	return orchestration, nil
}
