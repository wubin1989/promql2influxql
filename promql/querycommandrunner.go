package promql

import (
	"context"
	"encoding/json"
	influxdb "github.com/influxdata/influxdb1-client/v2"
	"github.com/influxdata/influxql"
	"github.com/pkg/errors"
	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/promql/parser"
	"github.com/unionj-cloud/go-doudou/v2/toolkit/stringutils"
	"github.com/unionj-cloud/go-doudou/v2/toolkit/zlogger"
	"github.com/wubin1989/promql2influxql/command"
	"github.com/wubin1989/promql2influxql/config"
	"github.com/wubin1989/promql2influxql/promql/evaluator"
	"github.com/wubin1989/promql2influxql/promql/transpiler"
	"sync"
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
			Config: cfg,
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
	config.Config
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

type RunResult struct {
	Result     parser.Value
	ResultType string
	Error      error
}

func (receiver *QueryCommandRunner) Run(ctx context.Context, cmd command.Command) (interface{}, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:

	}
	timeoutCtx, cancel := context.WithTimeout(ctx, receiver.Cfg.Timeout)
	defer cancel()
	resultChan := make(chan RunResult)
	go func() {
		handleErr := func(err error) {
			resultChan <- RunResult{
				Error: err,
			}
		}
		expr, err := parser.ParseExpr(cmd.Cmd)
		if err != nil {
			handleErr(errors.Wrap(err, "command parse fail"))
			return
		}
		t := &transpiler.Transpiler{
			Command: cmd,
		}
		node, err := t.Transpile(expr)
		if err != nil {
			handleErr(errors.Wrap(err, "command execute fail"))
			return
		}
		influxCmd := node.String()
		if receiver.Cfg.Verbose {
			zlogger.Info().Msgf("PromQL: %s => InfluxQL: %s", cmd.Cmd, influxCmd)
		}
		switch n := node.(type) {
		case influxql.Expr:
			if transpiler.YieldsFloat(expr) {
				var e evaluator.Evaluator
				n = e.EvalYieldsFloatExpr(expr)
			}
			switch expr := n.(type) {
			case influxql.Literal:
				result, resultType := receiver.InfluxLiteralToPromQLValue(expr, cmd)
				resultChan <- RunResult{
					Result:     result,
					ResultType: resultType,
				}
				return
			default:
				handleErr(transpiler.ErrPromExprNotSupported)
				return
			}
		case influxql.Statement:
			resp, err := receiver.Client.Query(influxdb.NewQuery(influxCmd, cmd.Database, ""))
			if err != nil {
				handleErr(errors.Wrap(err, "error from influxdb api"))
				return
			}
			if receiver.Cfg.Verbose {
				jsonResp, _ := json.Marshal(resp)
				zlogger.Info().RawJSON("response", jsonResp).Str("influxql", influxCmd).Str("promql", cmd.Cmd).Msg("response from InfluxDB")
			}
			if stringutils.IsNotEmpty(resp.Err) {
				handleErr(errors.Errorf("error from influxdb api: %s", resp.Err))
				return
			}
			result, resultType, err := receiver.InfluxResultToPromQLValue(resp.Results, expr, cmd)
			if err != nil {
				handleErr(errors.Wrap(err, "fail to convert result from influxdb format to native prometheus format"))
				return
			}
			resultChan <- RunResult{
				Result:     result,
				ResultType: resultType,
			}
			return
		default:
			handleErr(transpiler.ErrPromExprNotSupported)
			return
		}
	}()
	for {
		select {
		case <-timeoutCtx.Done():
			return nil, timeoutCtx.Err()
		case runResult := <-resultChan:
			if runResult.Error != nil {
				return nil, runResult.Error
			}
			return runResult, nil
		}
	}
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
