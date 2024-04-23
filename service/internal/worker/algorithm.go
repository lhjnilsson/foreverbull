package worker

import (
	"fmt"
	"sync"

	finance "github.com/lhjnilsson/foreverbull/finance/entity"
	"github.com/lhjnilsson/foreverbull/service/entity"
)

type Algorithm struct {
	entity *entity.ServiceAlgorithm
}

func NewAlgorithm(entity *entity.ServiceAlgorithm) *Algorithm {
	return &Algorithm{entity: entity}
}

func (a *Algorithm) GetFunctionChannel() <-chan entity.ServiceFunction {
	order := make(chan entity.ServiceFunction)
	go func() {
		defer close(order)
		var nextInputKey string
		for _, f := range a.entity.Functions {
			if f.InputKey == "symbols" {
				order <- f
				if f.ReturnType != "NAMESPACE_VALUE" {
					return
				}
				nextInputKey = *f.NamespaceReturnKey
				break
			}
		}
		for {
			for _, f := range a.entity.Functions {
				if f.InputKey == nextInputKey {
					order <- f
					if f.ReturnType != "NAMESPACE_VALUE" {
						return
					}
					nextInputKey = *f.NamespaceReturnKey
					break
				}
			}
		}
	}()
	return order
}

func AddToNamespace(namespace Namespace, function entity.ServiceFunction, data interface{}) error {
	return nil
}

type NamespaceDataTypes interface {
	string | float64 | finance.Order
}

type NamespaceObject[T NamespaceDataTypes] struct {
	data map[string]T
	mu   sync.Mutex
}

func (n *NamespaceObject[T]) AddData(data interface{}) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	switch data.(type) {
	case map[string]float64:
		data, ok := data.(map[string]T)
		if !ok {
			return fmt.Errorf("data is not a correct type")
		}
		for k, v := range data {
			data[k] = v
		}
	}
	return nil
}

func (n *NamespaceObject[T]) GetData() interface{} {
	return n.data
}

type NamespaceArray[T NamespaceDataTypes] struct {
	data []T
	mu   sync.Mutex
}

func (n *NamespaceArray[T]) AddData(data interface{}) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	d, ok := data.([]T)
	if !ok {
		return fmt.Errorf("data is not a correct type")
	}
	n.data = append(n.data, d...)
	return nil
}

func (n *NamespaceArray[T]) GetData() interface{} {
	return n.data
}

type Namespace interface {
	AddData(interface{}) error
	GetData() interface{}
}
