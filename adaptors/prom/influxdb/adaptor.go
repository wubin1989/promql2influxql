package influxdb

import (
	"context"
	_ "github.com/influxdata/influxdb1-client" // this is important because of the bug in go mod
	influxdb "github.com/influxdata/influxdb1-client/v2"
	"github.com/wubin1989/promql2influxql/applications"
	"time"
)

const (
	DefaultQueryTimeout = "2m"
)

// AdaptorConfig configures promql2influxql.InfluxDBAdaptor
type AdaptorConfig struct {
	// Timeout sets timeout duration for a single query execution
	Timeout time.Duration
	// Verbose indicates whether to output more logs or not
	Verbose bool
}

var _ applications.IPromAdaptor = (*Adaptor)(nil)

// Adaptor is a concrete struct that implementing IAdaptor.
// It depends on influxdb.Client to issue http requests to InfluxDB storage to fetch matrix data under the hood.
type Adaptor struct {
	_      [0]int
	Cfg    AdaptorConfig
	Client influxdb.Client
}

// Query implements IAdaptor's Query method
func (receiver *Adaptor) Query(ctx context.Context, cmd applications.PromCommand) (interface{}, error) {
	runner := queryCommandRunnerFactory.Build(receiver.Client, receiver.Cfg)
	defer runner.Recycle()
	return runner.Run(ctx, cmd)
}

// NewAdaptor is a package-level factory method to return a pointer to InfluxDBAdaptor.
// It is usually called by adaptor service on the upper layer.
func NewAdaptor(cfg AdaptorConfig, client influxdb.Client) *Adaptor {
	adaptor := Adaptor{
		Cfg:    cfg,
		Client: client,
	}
	return &adaptor
}
