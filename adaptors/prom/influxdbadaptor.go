package prom

import (
	"context"
	_ "github.com/influxdata/influxdb1-client" // this is important because of the bug in go mod
	influxdb "github.com/influxdata/influxdb1-client/v2"
	influx "github.com/wubin1989/promql2influxql/adaptors/prom/influxdb"
	"github.com/wubin1989/promql2influxql/applications"
	"time"
)

// InfluxDBAdaptorConfig configures prom.InfluxDBAdaptor
type InfluxDBAdaptorConfig struct {
	// Timeout sets timeout duration for a single query execution
	Timeout time.Duration
	// Verbose indicates whether to output more logs or not
	Verbose bool
}

var _ applications.IPromAdaptor = (*InfluxDBAdaptor)(nil)

// InfluxDBAdaptor is a concrete struct that implementing applications.IPromAdaptor.
// It depends on influxdb.Client to issue http requests to InfluxDB storage to fetch matrix data under the hood.
type InfluxDBAdaptor struct {
	_      [0]int
	Cfg    InfluxDBAdaptorConfig
	Client influxdb.Client
}

// Query implements applications.IPromAdaptor's Query method
func (receiver *InfluxDBAdaptor) Query(ctx context.Context, cmd applications.PromCommand) (applications.RunResult, error) {
	runner := influx.SingletonQueryCommandRunnerFactory.Build(receiver.Client, influx.QueryCommandRunnerConfig{
		Timeout: receiver.Cfg.Timeout,
		Verbose: receiver.Cfg.Verbose,
	})
	defer runner.Recycle()
	return runner.Run(ctx, cmd)
}

// NewInfluxDBAdaptor is a package-level factory method to return a pointer to InfluxDBAdaptor.
// It is usually called by application service on the upper layer.
func NewInfluxDBAdaptor(cfg InfluxDBAdaptorConfig, client influxdb.Client) *InfluxDBAdaptor {
	adaptor := InfluxDBAdaptor{
		Cfg:    cfg,
		Client: client,
	}
	return &adaptor
}
