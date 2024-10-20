package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Message interface {
	GetID() string
	GetOrchestrationID() string
	GetOrchestrationStep() string
	RawPayload() []byte
	ParsePayload(payload interface{}) error
	Call(ctx context.Context, key Dependency) (interface{}, error)
	MustGet(key Dependency) interface{}
}

func NewMessage(module, component, method string, entity any) (Message, error) {
	payload, err := json.Marshal(entity)
	if err != nil {
		return nil, fmt.Errorf("error marshalling payload: %w", err)
	}

	return &message{
		Module:    module,
		Component: component,
		Method:    method,
		Payload:   payload,
	}, nil
}

type messageStatus struct {
	Status     MessageStatus
	Error      *string
	OccurredAt time.Time
}

type message struct {
	ID                        *string
	OrchestrationName         *string
	OrchestrationID           *string
	OrchestrationStep         *string
	OrchestrationStepNumber   *int
	OrchestrationFallbackStep *bool

	Module              string
	Component           string
	Method              string
	Error               *string
	Payload             []byte
	StatusHistory       []messageStatus
	dependencyContainer *dependencyContainer
}

func (m *message) GetID() string {
	if m.ID == nil {
		return ""
	}

	return *m.ID
}

func (m *message) GetOrchestrationID() string {
	if m.OrchestrationID == nil {
		return ""
	}

	return *m.OrchestrationID
}

func (m *message) GetOrchestrationStep() string {
	if m.OrchestrationStep == nil {
		return ""
	}

	return *m.OrchestrationStep
}

func (m *message) RawPayload() []byte {
	return m.Payload
}

func (m *message) ParsePayload(v interface{}) error {
	if err := json.Unmarshal(m.Payload, v); err != nil {
		return fmt.Errorf("error unmarshalling payload: %w", err)
	}

	return nil
}

func (m *message) Call(ctx context.Context, key Dependency) (interface{}, error) {
	f, ok := m.dependencyContainer.methods[key]
	if !ok {
		return nil, fmt.Errorf("dependency not found: %s", key)
	}

	return f(ctx, m)
}

func (m *message) MustGet(key Dependency) interface{} {
	v, ok := m.dependencyContainer.singeltons[key]
	if !ok {
		panic(fmt.Sprintf("dependency not found: %s", key))
	}

	return v
}

type DependencyContainer interface {
	AddMethod(key Dependency, f func(context.Context, Message) (interface{}, error))
	AddSingleton(key Dependency, v interface{})
}

type dependencyContainer struct {
	methods    map[Dependency]func(context.Context, Message) (interface{}, error)
	singeltons map[Dependency]interface{}
}

func (d *dependencyContainer) AddMethod(key Dependency, f func(context.Context, Message) (interface{}, error)) {
	d.methods[key] = f
}

func (d *dependencyContainer) AddSingleton(key Dependency, v interface{}) {
	d.singeltons[key] = v
}

func NewDependencyContainer() DependencyContainer {
	return &dependencyContainer{
		methods:    make(map[Dependency]func(context.Context, Message) (interface{}, error)),
		singeltons: make(map[Dependency]interface{}),
	}
}

func NewMessageOrchestration(name string) *MessageOrchestration {
	return &MessageOrchestration{
		Name:            name,
		OrchestrationID: uuid.New().String(),
		Steps:           []MessageOrchestrationStep{}}
}

type MessageOrchestration struct {
	Name            string
	OrchestrationID string
	Steps           []MessageOrchestrationStep
	FallbackStep    *MessageOrchestrationStep
}

func (mo *MessageOrchestration) AddStep(name string, commands []Message) {
	step := MessageOrchestrationStep{
		OrchestrationName: mo.Name,
		OrchestrationID:   mo.OrchestrationID,
		OrchestrationStep: name,
		Name:              name,
		Commands:          commands,
	}
	fallbackStep := false
	stepNumber := len(mo.Steps)

	for _, cmd := range step.Commands {
		msg := cmd.(*message)
		msg.OrchestrationID = &step.OrchestrationID
		msg.OrchestrationName = &step.OrchestrationName
		msg.OrchestrationStep = &step.OrchestrationStep
		msg.OrchestrationStepNumber = &stepNumber
		msg.OrchestrationFallbackStep = &fallbackStep
	}

	mo.Steps = append(mo.Steps, step)
}

func (mo *MessageOrchestration) SettFallback(commands []Message) {
	step := MessageOrchestrationStep{
		OrchestrationName: mo.Name,
		OrchestrationID:   mo.OrchestrationID,
		OrchestrationStep: "fallback",
		Name:              "fallback",
		Commands:          commands,
	}
	fallbackStep := true

	for _, cmd := range step.Commands {
		msg := cmd.(*message)
		msg.OrchestrationID = &step.OrchestrationID
		msg.OrchestrationName = &step.OrchestrationName
		msg.OrchestrationStep = &step.OrchestrationStep
		msg.OrchestrationFallbackStep = &fallbackStep
	}

	mo.FallbackStep = &step
}

type MessageOrchestrationStep struct {
	Name string

	OrchestrationID   string
	OrchestrationName string
	OrchestrationStep string
	Commands          []Message
}
