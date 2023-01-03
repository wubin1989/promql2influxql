package promql2influxql

import (
	_ "github.com/influxdata/influxdb1-client" // this is important because of the bug in go mod
	influxdb "github.com/influxdata/influxdb1-client/v2"
	"github.com/wubin1989/promql2influxql/bizerrors"
	"github.com/wubin1989/promql2influxql/command"
)

type InfluxDBAdaptorConfig struct {
}

func assertInfluxDBAdaptorNotNil(adaptor *InfluxDBAdaptor) {
	if adaptor == nil {
		panic("please create InfluxDBAdaptor first")
	}
}

var _ IAdaptor = (*InfluxDBAdaptor)(nil)

type InfluxDBAdaptor struct {
	_      [0]int
	Cfg    InfluxDBAdaptorConfig
	Client influxdb.Client
}

func (receiver *InfluxDBAdaptor) Query(c command.Command) (command.CommandResult, error) {
	assertInfluxDBAdaptorNotNil(receiver)
	factory, ok := command.CommandRunnerFactoryRegistry.Factory(c.Dialect)
	if !ok {
		return command.CommandResult{}, bizerrors.DialectNotSupportedErr
	}
	return factory.Build(receiver.Client).Query(c.Cmd)
}

func (receiver *InfluxDBAdaptor) Write(c command.Command) error {
	assertInfluxDBAdaptorNotNil(receiver)
	factory, ok := command.CommandRunnerFactoryRegistry.Factory(c.Dialect)
	if !ok {
		return bizerrors.DialectNotSupportedErr
	}
	return factory.Build(receiver.Client).Write(c.Cmd)
}

func NewInfluxDBAdaptor(cfg InfluxDBAdaptorConfig, client influxdb.Client) *InfluxDBAdaptor {
	adaptor := InfluxDBAdaptor{
		Cfg:    cfg,
		Client: client,
	}
	return &adaptor
}
