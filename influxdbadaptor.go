package promql2influxql

import (
	"context"
	_ "github.com/influxdata/influxdb1-client" // this is important because of the bug in go mod
	influxdb "github.com/influxdata/influxdb1-client/v2"
	"github.com/wubin1989/promql2influxql/command"
	"github.com/wubin1989/promql2influxql/config"
)

var _ IAdaptor = (*InfluxDBAdaptor)(nil)

// InfluxDBAdaptor is a concrete struct that implementing IAdaptor.
// It depends on influxdb.Client to issue http requests to InfluxDB storage to fetch matrix data under the hood.
type InfluxDBAdaptor struct {
	_      [0]int
	Cfg    config.Config
	Client influxdb.Client
}

// Query implements IAdaptor's Query method
func (receiver *InfluxDBAdaptor) Query(ctx context.Context, cmd command.Command) (interface{}, error) {
	factory, ok := command.CommandRunnerFactoryRegistry.Factory(command.CommandType{
		OperationType: command.QUERY_OPERATION,
		DialectType:   cmd.Dialect,
	})
	if !ok {
		return nil, command.ErrDialectNotSupported
	}
	runner := factory.Build(receiver.Client, receiver.Cfg)
	if reusableRunner, ok := runner.(command.IReusableCommandRunner); ok {
		defer reusableRunner.Recycle()
	}
	return runner.Run(ctx, cmd)
}

// NewInfluxDBAdaptor is a package-level factory method to return a pointer to InfluxDBAdaptor.
// It is usually called by adaptor service on the upper layer.
func NewInfluxDBAdaptor(cfg config.Config, client influxdb.Client) *InfluxDBAdaptor {
	adaptor := InfluxDBAdaptor{
		Cfg:    cfg,
		Client: client,
	}
	return &adaptor
}
