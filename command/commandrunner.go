package command

import influxdb "github.com/influxdata/influxdb1-client/v2"

type ICommandRunner interface {
	Query(cmd string) (CommandResult, error)
	Write(cmd string) error
}

type ICommandRunnerFactory interface {
	Build(client influxdb.Client) ICommandRunner
}

var CommandRunnerFactoryRegistry *commandRunnerFactoryRegistry

func init() {
	CommandRunnerFactoryRegistry = &commandRunnerFactoryRegistry{
		Runners: make(map[DialectType]ICommandRunnerFactory),
	}
}

type commandRunnerFactoryRegistry struct {
	Runners map[DialectType]ICommandRunnerFactory
}

func (receiver *commandRunnerFactoryRegistry) Register(dialectType DialectType, factory ICommandRunnerFactory) {
	receiver.Runners[dialectType] = factory
}

func (receiver *commandRunnerFactoryRegistry) Factory(dialectType DialectType) (factory ICommandRunnerFactory, ok bool) {
	factory, ok = receiver.Runners[dialectType]
	return
}
