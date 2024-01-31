package stream

type Dependency string

const (
	LoggerDep Dependency = "logger"
	DBDep     Dependency = "db"
	StreamDep Dependency = "stream"
	ConfigDep Dependency = "config"
)
