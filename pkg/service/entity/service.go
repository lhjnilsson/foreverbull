package entity

import (
	"fmt"
	"time"
)

type ServiceStatusType string

const (
	ServiceStatusCreated   ServiceStatusType = "CREATED"
	ServiceStatusInterview ServiceStatusType = "INTERVIEW"
	ServiceStatusReady     ServiceStatusType = "READY"
	ServiceStatusError     ServiceStatusType = "ERROR"
)

type FunctionParameter struct {
	Key     string  `json:"key" mapstructure:"key"`
	Default *string `json:"default" mapstructure:"default"`
	Type    string  `json:"type" mapstructure:"type"`
}

type AlgorithmFunction struct {
	Name              string              `json:"name" mapstructure:"name"`
	Parameters        []FunctionParameter `json:"parameters" mapstructure:"parameters"`
	ParallelExecution bool                `json:"parallel_execution" mapstructure:"parallel_execution"`
	RunFirst          bool                `json:"run_first" mapstructure:"run_first"`
	RunLast           bool                `json:"run_last" mapstructure:"run_last"`
}

type Algorithm struct {
	FilePath   string              `json:"file_path" mapstructure:"file_path"`
	Functions  []AlgorithmFunction `json:"functions" mapstructure:"functions"`
	Namespaces []string            `json:"namespaces" mapstructure:"namespaces"`
}

func (a *Algorithm) Configure() (map[string]InstanceFunction, error) {
	functions := map[string]InstanceFunction{}
	for _, function := range a.Functions {
		parameters := map[string]string{}
		for _, parameter := range function.Parameters {
			if parameter.Default == nil {
				return nil, fmt.Errorf("parameter %s has no default value", parameter.Key)
			}
			parameters[parameter.Key] = *parameter.Default
		}
		functions[function.Name] = InstanceFunction{Parameters: parameters}
	}
	return functions, nil

}

type ServiceStatus struct {
	Status     ServiceStatusType `json:"status"`
	Error      *string           `json:"message"`
	OccurredAt time.Time         `json:"occurred_at"`
}

type Service struct {
	Image     string     `json:"image" binding:"required"`
	Algorithm *Algorithm `json:"algorithm" binding:"required"`

	Statuses []ServiceStatus `json:"statuses"`
}
