package command

import (
	"context"
	influxdb "github.com/influxdata/influxdb1-client/v2"
	"github.com/wubin1989/promql2influxql/config"
)

type ICommandRunner interface {
	Run(ctx context.Context, cmd string) (CommandResult, error)
}

type IReusableCommandRunner interface {
	ICommandRunner
	Recycle()
}

type ICommandRunnerFactory interface {
	Build(client influxdb.Client, cfg config.Config) ICommandRunner
}

var CommandRunnerFactoryRegistry *commandRunnerFactoryRegistry

func init() {
	CommandRunnerFactoryRegistry = &commandRunnerFactoryRegistry{
		Runners: make(map[CommandType]ICommandRunnerFactory),
	}
}

type commandRunnerFactoryRegistry struct {
	Runners map[CommandType]ICommandRunnerFactory
}

func (receiver *commandRunnerFactoryRegistry) Register(commandType CommandType, factory ICommandRunnerFactory) {
	receiver.Runners[commandType] = factory
}

func (receiver *commandRunnerFactoryRegistry) Factory(commandType CommandType) (factory ICommandRunnerFactory, ok bool) {
	factory, ok = receiver.Runners[commandType]
	return
}
