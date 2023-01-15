package influxdb

import (
	"context"
	"encoding/json"
	influxdb "github.com/influxdata/influxdb1-client/v2"
	"github.com/influxdata/influxql"
	"github.com/pkg/errors"
	"github.com/prometheus/prometheus/promql/parser"
	"github.com/unionj-cloud/go-doudou/v2/toolkit/stringutils"
	"github.com/unionj-cloud/go-doudou/v2/toolkit/zlogger"
	"github.com/wubin1989/promql2influxql/adaptors/prom/influxdb/evaluator"
	"github.com/wubin1989/promql2influxql/adaptors/prom/influxdb/transpiler"
	"github.com/wubin1989/promql2influxql/applications"
	"sync"
	"time"
)

var SingletonQueryCommandRunnerFactory *QueryCommandRunnerFactory

// init initializes a package-level QueryCommandRunnerFactory singleton.
func init() {
	SingletonQueryCommandRunnerFactory = &QueryCommandRunnerFactory{
		pool: sync.Pool{
			New: func() interface{} {
				return &QueryCommandRunner{}
			},
		},
	}
}

// QueryCommandRunnerFactory is responsible for creating QueryCommandRunner instances.
// It contains an object pool for performance and memory effective.
type QueryCommandRunnerFactory struct {
	pool sync.Pool
}

// Build returns an QueryCommandRunner instance from object pool
func (receiver *QueryCommandRunnerFactory) Build(client influxdb.Client, cfg QueryCommandRunnerConfig) *QueryCommandRunner {
	runner := receiver.pool.Get().(*QueryCommandRunner)
	runner.ApplyOpts(QueryCommandRunnerOpts{
		Cfg:     cfg,
		Client:  client,
		Factory: receiver,
	})
	return runner
}

// Recycle puts *QueryCommandRunner back to object pool
func (receiver *QueryCommandRunnerFactory) Recycle(runner *QueryCommandRunner) {
	receiver.pool.Put(runner)
}

type QueryCommandRunnerConfig struct {
	// Timeout sets timeout duration for a single query execution
	Timeout time.Duration
	// Verbose indicates whether to output more logs or not
	Verbose bool
}

type QueryCommandRunnerOpts struct {
	Cfg     QueryCommandRunnerConfig
	Client  influxdb.Client
	Factory *QueryCommandRunnerFactory
}

// QueryCommandRunner is a query engine to run applications.PromCommand.
// It contains a reference to QueryCommandRunnerFactory instance for putting itself back to the factory from which it was born after work.
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

// handleExprTranspileResult evaluates influxql.Expr itself locally.
func (receiver *QueryCommandRunner) handleExprTranspileResult(cmd applications.PromCommand, expr parser.Expr, n influxql.Expr, resultChan chan applications.RunResult, handleErr func(err error)) {
	if transpiler.YieldsFloat(expr) {
		var e evaluator.Evaluator
		n = e.EvalYieldsFloatExpr(expr)
	}
	switch expr := n.(type) {
	case influxql.Literal:
		result, resultType := receiver.InfluxLiteralToPromQLValue(expr, cmd)
		resultChan <- applications.RunResult{
			Result:     result,
			ResultType: resultType,
		}
		return
	default:
		handleErr(transpiler.ErrPromExprNotSupported)
		return
	}
}

// handleStatementTranspileResult delegates remote InfluxDB server to evaluate InfluxQL statements for us with the help of influxdb.Client.
func (receiver *QueryCommandRunner) handleStatementTranspileResult(cmd applications.PromCommand, expr parser.Expr, influxCmd string, resultChan chan applications.RunResult, handleErr func(err error)) {
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
	var result interface{}
	var resultType string
	switch cmd.DataType {
	case applications.LABEL_VALUES_DATA:
		var dest []string
		if err = receiver.InfluxResultToStringSlice(resp.Results, &dest, expr, cmd); err != nil {
			handleErr(errors.Wrap(err, "fail to convert result from influxdb format to string slice"))
			return
		}
		result = dest[:]
	default:
		result, resultType, err = receiver.InfluxResultToPromQLValue(resp.Results, expr, cmd)
		if err != nil {
			handleErr(errors.Wrap(err, "fail to convert result from influxdb format to native prometheus format"))
			return
		}
	}
	resultChan <- applications.RunResult{
		Result:     result,
		ResultType: resultType,
	}
}

func (receiver *QueryCommandRunner) handleCmdNotEmptyCase(cmd applications.PromCommand, resultChan chan applications.RunResult, handleErr func(err error)) {
	switch cmd.DataType {
	case applications.LABEL_VALUES_DATA:
		// If cmd is applications.LABEL_VALUES_DATA, we should check whether the PromQL query expression is valid or not.
		// If not valid, return error immediately.
		if _, err := parser.ParseMetricSelector(cmd.Cmd); err != nil {
			handleErr(errors.Wrap(err, "to get label values the PromQL command must be vector selector"))
			return
		}
	default:
	}
	// Parse cmd.Cmd to PromQL ast
	expr, err := parser.ParseExpr(cmd.Cmd)
	if err != nil {
		handleErr(errors.Wrap(err, "command parse fail"))
		return
	}
	t := &transpiler.Transpiler{
		PromCommand: cmd,
	}
	// Transpile PromQL ast to InfluxQL ast
	node, err := t.Transpile(expr)
	if err != nil {
		handleErr(errors.Wrap(err, "command execute fail"))
		return
	}
	// Get string representation of InfluxQL query expression from the ast
	influxCmd := node.String()
	if receiver.Cfg.Verbose {
		zlogger.Info().Msgf("PromQL: %s => InfluxQL: %s", cmd.Cmd, influxCmd)
	}
	switch n := node.(type) {
	case influxql.Expr:
		// Evaluate influxql.Expr
		receiver.handleExprTranspileResult(cmd, expr, n, resultChan, handleErr)
	case influxql.Statement:
		// Evaluate influxql.Statement
		receiver.handleStatementTranspileResult(cmd, expr, influxCmd, resultChan, handleErr)
	default:
		handleErr(transpiler.ErrPromExprNotSupported)
	}
}

// Run executes applications.PromCommand and returns final results
func (receiver *QueryCommandRunner) Run(ctx context.Context, cmd applications.PromCommand) (applications.RunResult, error) {
	// check whether context.Context has been ended or not.
	// If yes, return immediately for saving resources.
	select {
	case <-ctx.Done():
		return applications.RunResult{}, ctx.Err()
	default:

	}
	timeoutCtx, cancel := context.WithTimeout(ctx, receiver.Cfg.Timeout)
	defer cancel()
	resultChan := make(chan applications.RunResult, 1)
	go func() {
		handleErr := func(err error) {
			resultChan <- applications.RunResult{
				Error: err,
			}
		}
		// If cmd is applications.LABEL_VALUES_DATA, cmd.Cmd may be empty.
		// Here we handle the other case.
		if stringutils.IsNotEmpty(cmd.Cmd) {
			receiver.handleCmdNotEmptyCase(cmd, resultChan, handleErr)
			return
		}
		// If cmd.Cmd is empty, we go here. We only handle applications.LABEL_VALUES_DATA case here currently.
		switch cmd.DataType {
		case applications.LABEL_VALUES_DATA:
			// It's a SHOW TAG VALUES statement.
			node := &influxql.ShowTagValuesStatement{
				Database:   cmd.Database,
				Op:         influxql.EQ,
				TagKeyExpr: &influxql.StringLiteral{Val: cmd.LabelName},
			}
			influxCmd := node.String()
			if receiver.Cfg.Verbose {
				zlogger.Info().Msgf("PromQL: %s => InfluxQL: %s", cmd.Cmd, influxCmd)
			}
			receiver.handleStatementTranspileResult(cmd, nil, influxCmd, resultChan, handleErr)
		default:
		}
	}()
	for {
		select {
		case <-timeoutCtx.Done():
			return applications.RunResult{}, timeoutCtx.Err()
		case runResult := <-resultChan:
			if runResult.Error != nil {
				return applications.RunResult{}, runResult.Error
			}
			return runResult, nil
		}
	}
}

// Recycle puts callee back to its factory
func (receiver *QueryCommandRunner) Recycle() {
	receiver.Factory.Recycle(receiver)
}
