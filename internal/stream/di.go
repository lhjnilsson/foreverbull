package stream

import "context"

type Dependency string

const (
	DBDep              Dependency = "db"
	StreamDep          Dependency = "stream"
	ContainerEngineDep Dependency = "container_engine"
	StorageDep         Dependency = "storage"
)

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
