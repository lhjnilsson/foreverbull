package stream

import (
	"fmt"

	"github.com/lhjnilsson/foreverbull/internal/stream"
)

type InstanceInterviewCommand struct {
	ID string
}

func NewInstanceInterviewCommand(instanceID string) (stream.Message, error) {
	entity := &InstanceInterviewCommand{
		ID: instanceID,
	}

	msg, err := stream.NewMessage("service", "instance", "interview", entity)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %v", err)
	}

	return msg, nil
}

type InstanceStopCommand struct {
	ID string
}

func NewInstanceStopCommand(instanceID string) (stream.Message, error) {
	entity := &InstanceStopCommand{
		ID: instanceID,
	}

	msg, err := stream.NewMessage("service", "instance", "stop", entity)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %v", err)
	}

	return msg, nil
}

type InstanceSanityCheckCommand struct {
	IDs []string
}

func NewInstanceSanityCheckCommand(instanceIDs []string) (stream.Message, error) {
	entity := &InstanceSanityCheckCommand{
		IDs: instanceIDs,
	}

	msg, err := stream.NewMessage("service", "instance", "sanity_check", entity)
	if err != nil {
		return nil, fmt.Errorf("error creating message: %v", err)
	}

	return msg, nil
}
