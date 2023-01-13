package command

import (
	"context"
	influxdb "github.com/influxdata/influxdb1-client/v2"
	"github.com/wubin1989/promql2influxql/config"
)

// ICommandRunner is an interface for query engine
type ICommandRunner interface {
	Run(ctx context.Context, cmd Command) (interface{}, error)
}

// IReusableCommandRunner is an interface for query engine and supporting reuse itself
type IReusableCommandRunner interface {
	ICommandRunner
	Recycle()
}

// ICommandRunnerFactory is an interface for query engine factory
type ICommandRunnerFactory interface {
	Build(client influxdb.Client, cfg config.Config) ICommandRunner
}

// CommandRunnerFactoryRegistry is the global query engine factory registry
var CommandRunnerFactoryRegistry *commandRunnerFactoryRegistry

func init() {
	CommandRunnerFactoryRegistry = &commandRunnerFactoryRegistry{
		Runners: make(map[CommandType]ICommandRunnerFactory),
	}
}

type commandRunnerFactoryRegistry struct {
	Runners map[CommandType]ICommandRunnerFactory
}

// Register registers new singleton query engine factory into each CommandType buckets
func (receiver *commandRunnerFactoryRegistry) Register(commandType CommandType, factory ICommandRunnerFactory) {
	receiver.Runners[commandType] = factory
}

// Factory returns a query engine factory from registry
func (receiver *commandRunnerFactoryRegistry) Factory(commandType CommandType) (factory ICommandRunnerFactory, ok bool) {
	factory, ok = receiver.Runners[commandType]
	return
}
