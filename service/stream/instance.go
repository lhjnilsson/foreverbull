package stream

import (
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/service/entity"
)

type InstanceMessage entity.Instance

type InstanceInterviewCommand struct {
	ID string `json:"id"`
}

func NewInstanceInterviewCommand(instanceID string) (stream.Message, error) {
	entity := &InstanceInterviewCommand{
		ID: instanceID,
	}
	return stream.NewMessage("service", "instance", "interview", entity)
}

type InstanceStopCommand struct {
	ID string `json:"id"`
}

func NewInstanceStopCommand(instanceID string) (stream.Message, error) {
	entity := &InstanceStopCommand{
		ID: instanceID,
	}
	return stream.NewMessage("service", "instance", "stop", entity)
}

type InstanceSanityCheckCommand struct {
	IDs []string `json:"ids"`
}

func NewInstanceSanityCheckCommand(instanceIDs []string) (stream.Message, error) {
	entity := &InstanceSanityCheckCommand{
		IDs: instanceIDs,
	}
	return stream.NewMessage("service", "instance", "sanity_check", entity)
}
