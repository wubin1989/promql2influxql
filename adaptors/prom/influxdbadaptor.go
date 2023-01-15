package prom

import (
	"context"
	_ "github.com/influxdata/influxdb1-client" // this is important because of the bug in go mod
	influxdb "github.com/influxdata/influxdb1-client/v2"
	"github.com/pkg/errors"
	"github.com/unionj-cloud/go-doudou/v2/toolkit/caller"
	influx "github.com/wubin1989/promql2influxql/adaptors/prom/influxdb"
	"github.com/wubin1989/promql2influxql/adaptors/prom/models"
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
	promCommand := models.PromCommand{
		Cmd:           cmd.Cmd,
		Database:      cmd.Database,
		Start:         cmd.Start,
		End:           cmd.End,
		Timezone:      cmd.Timezone,
		Evaluation:    cmd.Evaluation,
		Step:          cmd.Step,
		DataType:      models.DataType(cmd.DataType),
		ValueFieldKey: cmd.ValueFieldKey,
		LabelName:     cmd.LabelName,
	}
	runResult, err := runner.Run(ctx, promCommand)
	if err != nil {
		return applications.RunResult{}, errors.Wrap(err, caller.NewCaller().String())
	}
	return applications.RunResult{
		Result:     runResult.Result,
		ResultType: runResult.ResultType,
		Error:      runResult.Error,
	}, nil
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
