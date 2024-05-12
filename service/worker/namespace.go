package worker

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/lhjnilsson/foreverbull/service/entity"
)

type containerDataTypes interface {
	string | float64 | int | bool
}

type container interface {
	Get() interface{}
	Set(value interface{}) error
	Flush()
}

type objectContainer[T containerDataTypes] struct {
	items map[string]T
	mu    sync.Mutex
}

func (n *objectContainer[T]) Get() interface{} {
	return n.items
}

func (n *objectContainer[T]) Set(value interface{}) error {
	obj := map[string]T{}
	switch v := value.(type) {
	case string:
		err := json.Unmarshal([]byte(v), &obj)
		if err != nil {
			return fmt.Errorf("failed to decode value: %w", err)
		}
	case []byte:
		err := json.Unmarshal(v, &obj)
		if err != nil {
			return fmt.Errorf("failed to decode value: %w", err)
		}

	default:
		return fmt.Errorf("unsupported type: %T", value)
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	for key, value := range obj {
		n.items[key] = value
	}
	return nil
}

func (n *objectContainer[T]) Flush() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.items = map[string]T{}
}

type arrayContainer[T containerDataTypes] struct {
	items []T
	mu    sync.Mutex
}

func (n *arrayContainer[T]) Get() interface{} {
	return n.items
}

func (n *arrayContainer[T]) Set(value interface{}) error {
	obj := []T{}
	switch v := value.(type) {
	case string:
		err := json.Unmarshal([]byte(v), &obj)
		if err != nil {
			return fmt.Errorf("failed to decode value: %w", err)
		}
	case []byte:
		err := json.Unmarshal(v, &obj)
		if err != nil {
			return fmt.Errorf("failed to decode value: %w", err)
		}
	default:
		return fmt.Errorf("unsupported type: %T", value)
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	n.items = append(n.items, obj...)
	return nil
}

func (n *arrayContainer[T]) Flush() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.items = []T{}
}

func CreateNamespace(algo *entity.Algorithm) (*namespace, error) {
	n := &namespace{
		containers: map[string]container{},
	}
	for key, value := range algo.Namespace {
		switch value.ValueType {
		}

		switch value.Type {
		case "object":
			switch value.ValueType {
			case "string":
				n.containers[key] = &objectContainer[string]{items: map[string]string{}}
			case "float64":
				n.containers[key] = &objectContainer[float64]{items: map[string]float64{}}
			case "float":
				n.containers[key] = &objectContainer[float64]{items: map[string]float64{}}
			case "int":
				n.containers[key] = &objectContainer[int]{items: map[string]int{}}
			case "bool":
				n.containers[key] = &objectContainer[bool]{items: map[string]bool{}}
			default:
				return nil, fmt.Errorf("unsupported value type: %s", value.ValueType)
			}
		case "array":
			switch value.ValueType {
			case "string":
				n.containers[key] = &arrayContainer[string]{items: []string{}}
			case "float64":
				n.containers[key] = &arrayContainer[float64]{items: []float64{}}
			case "float":
				n.containers[key] = &arrayContainer[float64]{items: []float64{}}
			case "int":
				n.containers[key] = &arrayContainer[int]{items: []int{}}
			case "bool":
				n.containers[key] = &arrayContainer[bool]{items: []bool{}}
			default:
				return nil, fmt.Errorf("unsupported value type: %s", value.ValueType)
			}
		}
	}
	return n, nil
}

type namespace struct {
	containers map[string]container
}

func (n *namespace) Set(key string, value interface{}) error {
	container, ok := n.containers[key]
	if !ok {
		return fmt.Errorf("namespace key not found: %s", key)
	}
	return container.Set(value)
}

func (n *namespace) Get(key string) (interface{}, error) {
	container, ok := n.containers[key]
	if !ok {
		return nil, fmt.Errorf("namespace key not found: %s", key)
	}
	return container.Get(), nil
}

func (n *namespace) Flush() {
	for _, container := range n.containers {
		container.Flush()
	}
}
