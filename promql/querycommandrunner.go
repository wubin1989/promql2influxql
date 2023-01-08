package promql

import (
	"context"
	influxdb "github.com/influxdata/influxdb1-client/v2"
	"github.com/pkg/errors"
	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/promql/parser"
	"github.com/unionj-cloud/go-doudou/v2/toolkit/stringutils"
	"github.com/wubin1989/promql2influxql/command"
	"github.com/wubin1989/promql2influxql/config"
	"github.com/wubin1989/promql2influxql/promql/transpiler"
	"sync"
	"time"
)

const (
	PROMQL_DIALECT command.DialectType = "promql"
)

const queryExecutionStr = "query execution"

func init() {
	command.CommandRunnerFactoryRegistry.Register(command.CommandType{
		OperationType: command.QUERY_OPERATION,
		DialectType:   PROMQL_DIALECT,
	}, NewQueryCommandRunnerFactory())
}

var _ command.ICommandRunnerFactory = (*QueryCommandRunnerFactory)(nil)

type QueryCommandRunnerFactory struct {
	pool sync.Pool
}

func (receiver *QueryCommandRunnerFactory) Build(client influxdb.Client, cfg config.Config) command.ICommandRunner {
	runner := receiver.pool.Get().(*QueryCommandRunner)
	runner.ApplyOpts(QueryCommandRunnerOpts{
		Cfg: QueryCommandRunnerConfig{
			MaxSamples: cfg.MaxSamples,
			Timeout:    cfg.Timeout,
		},
		Client:  client,
		Factory: receiver,
	})
	return runner
}

func (receiver *QueryCommandRunnerFactory) Recycle(runner *QueryCommandRunner) {
	receiver.pool.Put(runner)
}

func NewQueryCommandRunnerFactory() *QueryCommandRunnerFactory {
	return &QueryCommandRunnerFactory{
		pool: sync.Pool{
			New: func() interface{} {
				return &QueryCommandRunner{}
			},
		},
	}
}

type QueryCommandRunnerConfig struct {
	MaxSamples           int
	Timeout              time.Duration
	EnableAtModifier     bool
	EnableNegativeOffset bool
}

type QueryCommandRunnerOpts struct {
	Cfg     QueryCommandRunnerConfig
	Client  influxdb.Client
	Factory *QueryCommandRunnerFactory
}

var _ command.ICommandRunner = (*QueryCommandRunner)(nil)
var _ command.IReusableCommandRunner = (*QueryCommandRunner)(nil)

type QueryCommandRunner struct {
	Cfg     QueryCommandRunnerConfig
	Client  influxdb.Client
	Factory *QueryCommandRunnerFactory
}

func (receiver *QueryCommandRunner) ApplyOpts(opts QueryCommandRunnerOpts) {
	receiver.Cfg = opts.Cfg
	receiver.Client = opts.Client
	receiver.Factory = opts.Factory
}

func (receiver *QueryCommandRunner) Run(ctx context.Context, cmd command.Command) (interface{}, error) {
	expr, err := parser.ParseExpr(cmd.Cmd)
	if err != nil {
		return nil, errors.Wrap(err, "command parse fail")
	}
	t := transpiler.NewTranspiler(cmd.Start, cmd.End,
		transpiler.WithTimezone(cmd.Timezone),
		transpiler.WithEvaluation(cmd.Evaluation),
		transpiler.WithStep(cmd.Step),
		transpiler.WithDataType(transpiler.DataType(cmd.DataType)),
	)
	sql, err := t.Transpile(expr)
	if err != nil {
		return nil, errors.Wrap(err, "command execute fail")
	}
	resp, err := receiver.Client.Query(influxdb.NewQuery(sql, cmd.Database, ""))
	if err != nil {
		return nil, errors.Wrap(err, "error from influxdb api")
	}
	if stringutils.IsNotEmpty(resp.Err) {
		return nil, errors.Errorf("error from influxdb api: %s", resp.Err)
	}
	// TODO properly handle parser.ValueType
	result, err := InfluxResultToPromQLValue(resp.Results, parser.ValueTypeMatrix)
	if err != nil {
		return nil, errors.Wrap(err, "fail to convert result from influxdb format to native prometheus format")
	}
	return result, nil
}

func (receiver *QueryCommandRunner) Recycle() {
	receiver.Factory.Recycle(receiver)
}

func contextErr(err error, msg string) error {
	switch {
	case errors.Is(err, context.Canceled):
		return promql.ErrQueryCanceled(msg)
	case errors.Is(err, context.DeadlineExceeded):
		return promql.ErrQueryTimeout(msg)
	default:
		return err
	}
}
