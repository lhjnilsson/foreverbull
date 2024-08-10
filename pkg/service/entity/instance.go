package entity

import (
	"fmt"
	"time"

	"github.com/golang/protobuf/proto"
	service_pb "github.com/lhjnilsson/foreverbull/internal/pb/service"
	"github.com/lhjnilsson/foreverbull/internal/socket"
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

func (i *Instance) GetInfo() (string, *Algorithm, error) {
	request := service_pb.Request{
		Task: "info",
	}
	response := service_pb.Response{}
	socket, err := i.GetSocket()
	if err != nil {
		return "", nil, fmt.Errorf("failed to get socket: %v", err)
	}
	if err := socket.Request(&request, &response); err != nil {
		return "", nil, fmt.Errorf("failed to send request: %v", err)
	}
	if response.Error != nil {
		return "", nil, fmt.Errorf("error in response: %s", *response.Error)
	}
	service_info_rsp := service_pb.ServiceInfoResponse{}
	if err := proto.Unmarshal(response.Data, &service_info_rsp); err != nil {
		return "", nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}
	var a *Algorithm
	if service_info_rsp.Algorithm != nil {
		algorithm := service_info_rsp.Algorithm
		a = &Algorithm{
			FilePath:   algorithm.FilePath,
			Namespaces: service_info_rsp.Algorithm.Namespaces,
		}
		for _, function := range algorithm.Functions {
			f := AlgorithmFunction{
				Name: function.Name,
			}
			for _, parameter := range function.Parameters {
				p := FunctionParameter{
					Key:     parameter.Key,
					Type:    parameter.ValueType,
					Default: parameter.DefaultValue,
				}
				f.Parameters = append(f.Parameters, p)
			}
			a.Functions = append(a.Functions, f)
		}
	}
	return service_info_rsp.ServiceType, a, nil
}

func (i *Instance) Configure(configuration *map[string]InstanceFunction) error {
	s, err := i.GetSocket()
	if err != nil {
		return fmt.Errorf("failed to get socket: %v", err)
	}
	configure_req := service_pb.ConfigureExecutionRequest{
		BrokerPort:    int32(*i.BrokerPort),
		NamespacePort: int32(*i.NamespacePort),
		DatabaseURL:   *i.DatabaseURL,
	}
	for key, value := range *configuration {
		f := service_pb.ConfigureExecutionRequest_Function{
			Name: key,
		}
		for k, v := range value.Parameters {
			param := service_pb.ConfigureExecutionRequest_FunctionParameter{
				Key:   k,
				Value: v,
			}
			f.Parameters = append(f.Parameters, &param)
		}
		configure_req.Functions = append(configure_req.Functions, &f)
	}
	data, err := proto.Marshal(&configure_req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %v", err)
	}
	request := service_pb.Request{
		Task: "configure_execution",
		Data: data,
	}
	response := service_pb.Response{}
	if err := s.Request(&request, &response, socket.WithReadTimeout(time.Second), socket.WithSendTimeout(time.Second)); err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	if response.Error != nil {
		return fmt.Errorf("error in response: %s", *response.Error)
	}

	request = service_pb.Request{
		Task: "run_execution",
	}
	response = service_pb.Response{}
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
	request := service_pb.Request{
		Task: "stop",
	}
	response := service_pb.Response{}
	if err := socket.Request(&request, &response); err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	if response.Error != nil {
		return fmt.Errorf("error in response: %s", *response.Error)
	}
	return nil
}
