package stream

import (
	"github.com/google/uuid"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/service/pb"
)

type UpdateServiceStatusCommand struct {
	Image  string                   `json:"image"`
	Status pb.Service_Status_Status `json:"status"`
	Error  error                    `json:"error"`
}

type ServiceStartCommand struct {
	Image      string `json:"image"`
	InstanceID string `json:"instance_id"`
}

func NewUpdateServiceStatusCommand(image string, status pb.Service_Status_Status, err error) (stream.Message, error) {
	entity := &UpdateServiceStatusCommand{
		Image:  image,
		Status: status,
		Error:  err,
	}
	return stream.NewMessage("service", "service", "status", entity)
}

func NewInstanceID() string {
	return uuid.New().String()
}

func NewServiceStartCommand(image, instanceID string) (stream.Message, error) {
	entity := &ServiceStartCommand{
		Image:      image,
		InstanceID: instanceID,
	}
	return stream.NewMessage("service", "service", "start", entity)
}

func NewServiceInterviewOrchestration(image string) (*stream.MessageOrchestration, error) {
	orchestration := stream.NewMessageOrchestration("service interview")

	instanceID := NewInstanceID()
	msg, err := NewServiceStartCommand(image, instanceID)
	if err != nil {
		return nil, err
	}
	msg2, err := NewUpdateServiceStatusCommand(image, pb.Service_Status_INTERVIEW, nil)
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
	msg2, err = NewUpdateServiceStatusCommand(image, pb.Service_Status_READY, nil)
	if err != nil {
		return nil, err
	}
	orchestration.AddStep("stop service", []stream.Message{msg, msg2})

	msg, err = NewInstanceStopCommand(instanceID)
	if err != nil {
		return nil, err
	}
	msg2, err = NewUpdateServiceStatusCommand(image, pb.Service_Status_ERROR, nil)
	if err != nil {
		return nil, err
	}
	orchestration.SettFallback([]stream.Message{msg, msg2})

	return orchestration, nil
}
