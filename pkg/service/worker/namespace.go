package worker

import (
	"fmt"
	"sync"

	"google.golang.org/protobuf/types/known/structpb"
)

type Namespace interface {
	Get(key string) *structpb.Struct
	Set(key string, value *structpb.Struct) error
	Flush()
}

type namespaceContainer struct {
	value *structpb.Struct
	sync  sync.Mutex
}

type namespace struct {
	values map[string]*namespaceContainer
}

func CreateNamespace(namespaces []string) *namespace {
	nspace := &namespace{
		values: make(map[string]*namespaceContainer),
	}
	for _, n := range namespaces {
		nspace.values[n] = &namespaceContainer{
			value: &structpb.Struct{
				Fields: make(map[string]*structpb.Value),
			},
		}
	}

	return nspace
}

func (n *namespace) Get(key string) *structpb.Struct {
	container, ok := n.values[key]
	if !ok {
		return nil
	}

	return container.value
}

func (n *namespace) Set(key string, value *structpb.Struct) error {
	container, ok := n.values[key]
	if !ok {
		return fmt.Errorf("namespace not found")
	}

	container.sync.Lock()
	defer container.sync.Unlock()

	for k, v := range value.Fields {
		container.value.Fields[k] = v
	}

	return nil
}

func (n *namespace) Flush() {
	for _, v := range n.values {
		v.sync.Lock()
		defer v.sync.Unlock()
		v.value = &structpb.Struct{
			Fields: make(map[string]*structpb.Value),
		}
	}
}
