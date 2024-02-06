package stream

type Dependency string

const (
	DBDep     Dependency = "db"
	StreamDep Dependency = "stream"
	ConfigDep Dependency = "config"
)
