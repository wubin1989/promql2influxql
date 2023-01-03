package promql

import (
	influxdb "github.com/influxdata/influxdb1-client/v2"
	"github.com/wubin1989/promql2influxql/command"
)

const (
	PROMQL_DIALECT command.DialectType = "promql"
)

func init() {
	command.CommandRunnerFactoryRegistry.Register(PROMQL_DIALECT, &PromQLCommandRunnerFactory{})
}

var _ command.ICommandRunner = (*PromQLCommandRunner)(nil)

type PromQLCommandRunner struct {
	Client influxdb.Client
}

func (p *PromQLCommandRunner) Query(cmd string) (command.CommandResult, error) {
	//TODO implement me
	panic("implement me")
}

func (p *PromQLCommandRunner) Write(cmd string) error {
	//TODO implement me
	panic("implement me")
}

var _ command.ICommandRunnerFactory = (*PromQLCommandRunnerFactory)(nil)

type PromQLCommandRunnerFactory struct {
	Client influxdb.Client
}

func (p *PromQLCommandRunnerFactory) Build(client influxdb.Client) command.ICommandRunner {
	return &PromQLCommandRunner{
		Client: client,
	}
}

func NewPromQLCommandRunnerFactory(client influxdb.Client) *PromQLCommandRunnerFactory {
	return &PromQLCommandRunnerFactory{
		Client: client,
	}
}
