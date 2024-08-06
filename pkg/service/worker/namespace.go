package worker

import (
	"fmt"
	"sync"

	"github.com/lhjnilsson/foreverbull/pkg/service/entity"
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

func CreateNamespace(namespaces map[string]entity.AlgorithmNamespace) *namespace {
	ns := &namespace{
		values: make(map[string]*namespaceContainer),
	}
	for k, _ := range namespaces {
		ns.values[k] = &namespaceContainer{
			value: &structpb.Struct{},
		}
	}
	return ns
}

func (n *namespace) Get(key string) *structpb.Struct {
	v, ok := n.values[key]
	if !ok {
		return nil
	}
	return v.value
}

func (n *namespace) Set(key string, value *structpb.Struct) error {
	c, ok := n.values[key]
	if !ok {
		return fmt.Errorf("namespace not found")
	}
	c.sync.Lock()
	defer c.sync.Unlock()
	for k, v := range value.Fields {
		c.value.Fields[k] = v
	}
	return nil
}

func (n *namespace) Flush() {
	for _, v := range n.values {
		v.sync.Lock()
		defer v.sync.Unlock()
		v.value = &structpb.Struct{}
	}
}
