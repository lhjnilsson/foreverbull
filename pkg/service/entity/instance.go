package entity

import (
	"fmt"
	"time"

	common_pb "github.com/lhjnilsson/foreverbull/internal/pb"
	"github.com/lhjnilsson/foreverbull/internal/socket"
	service_pb "github.com/lhjnilsson/foreverbull/pkg/service/pb"
	"google.golang.org/protobuf/proto"
)

type InstanceStatusType string

const (
	InstanceStatusCreated InstanceStatusType = "CREATED"
	InstanceStatusRunning InstanceStatusType = "RUNNING"
	InstanceStatusStopped InstanceStatusType = "STOPPED"
	InstanceStatusError   InstanceStatusType = "ERROR"
)

type InstanceFunction struct {
	Parameters map[string]string `json:"parameters" mapstructure:"parameters"`
}

type InstanceStatus struct {
	Status     InstanceStatusType `json:"status"`
	Error      *string            `json:"message"`
	OccurredAt time.Time          `json:"occurred_at"`
}

type Instance struct {
	ID    string  `json:"id"`
	Image *string `json:"image"`

	Host          *string `json:"host"`
	Port          *int    `json:"port"`
	BrokerPort    *int    `json:"broker_port" mapstructure:"broker_port"`
	NamespacePort *int    `json:"namespace_port" mapstructure:"namespace_port"`
	DatabaseURL   *string `json:"database_url" mapstructure:"database_url"`

	Statuses []InstanceStatus `json:"statuses"`
}

func (i *Instance) GetSocket() (socket.Requester, error) {
	if i.Host == nil || i.Port == nil {
		return nil, fmt.Errorf("instance %s has no host or port", i.ID)
	}
	return socket.NewRequester(*i.Host, *i.Port, true)
}

func (i *Instance) GetInfo() (*Algorithm, error) {
	request := common_pb.Request{
		Task: "info",
	}
	response := common_pb.Response{}
	socket, err := i.GetSocket()
	if err != nil {
		return nil, fmt.Errorf("failed to get socket: %v", err)
	}
	if err := socket.Request(&request, &response); err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	if response.Error != nil {
		return nil, fmt.Errorf("error in response: %s", *response.Error)
	}
	service_info_rsp := service_pb.GetServiceInfoResponse{}
	if err := proto.Unmarshal(response.Data, &service_info_rsp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	return nil, nil
}

func (i *Instance) Configure(configuration *map[string]InstanceFunction) error {
	s, err := i.GetSocket()
	if err != nil {
		return fmt.Errorf("failed to get socket: %v", err)
	}
	configure_req := service_pb.ConfigureExecutionRequest{
		Configuration: &service_pb.ExecutionConfiguration{
			BrokerPort:    int32(*i.BrokerPort),
			NamespacePort: int32(*i.NamespacePort),
			DatabaseURL:   *i.DatabaseURL,
		},
	}
	for key, value := range *configuration {
		f := service_pb.ExecutionConfiguration_Function{
			Name: key,
		}
		for k, v := range value.Parameters {
			param := service_pb.ExecutionConfiguration_FunctionParameter{
				Key:   k,
				Value: v,
			}
			f.Parameters = append(f.Parameters, &param)
		}
		configure_req.Configuration.Functions = append(configure_req.Configuration.Functions, &f)
	}
	data, err := proto.Marshal(&configure_req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %v", err)
	}
	request := common_pb.Request{
		Task: "configure_execution",
		Data: data,
	}
	response := common_pb.Response{}
	if err := s.Request(&request, &response, socket.WithReadTimeout(time.Second), socket.WithSendTimeout(time.Second)); err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	if response.Error != nil {
		return fmt.Errorf("error in response: %s", *response.Error)
	}

	request = common_pb.Request{
		Task: "run_execution",
	}
	response = common_pb.Response{}
	if err := s.Request(&request, &response, socket.WithReadTimeout(time.Second), socket.WithSendTimeout(time.Second)); err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	if response.Error != nil {
		return fmt.Errorf("error in response: %s", *response.Error)
	}
	return nil
}

func (i *Instance) Stop() error {
	socket, err := i.GetSocket()
	if err != nil {
		return fmt.Errorf("failed to get socket: %v", err)
	}
	request := common_pb.Request{
		Task: "stop",
	}
	response := common_pb.Response{}
	if err := socket.Request(&request, &response); err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	if response.Error != nil {
		return fmt.Errorf("error in response: %s", *response.Error)
	}
	return nil
}
