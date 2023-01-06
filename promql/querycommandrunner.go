package promql

import (
	"context"
	influxdb "github.com/influxdata/influxdb1-client/v2"
	"github.com/pkg/errors"
	"github.com/prometheus/prometheus/model/timestamp"
	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/promql/parser"
	"github.com/unionj-cloud/go-doudou/v2/toolkit/stringutils"
	"github.com/wubin1989/promql2influxql/command"
	"github.com/wubin1989/promql2influxql/config"
	"github.com/wubin1989/promql2influxql/promql/transpiler"
	"math"
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
	t := transpiler.NewTranspiler(cmd.Start, cmd.End, transpiler.WithTimezone(cmd.Timezone), transpiler.WithEvaluation(cmd.Evaluation))
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

// contextDone returns an error if the context was canceled or timed out.
func contextDone(ctx context.Context, msg string) error {
	if err := ctx.Err(); err != nil {
		return contextErr(err, msg)
	}
	return nil
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

func (receiver *QueryCommandRunner) findMinMaxTime(s *parser.EvalStmt) (int64, int64) {
	var minTimestamp, maxTimestamp int64 = math.MaxInt64, math.MinInt64
	// Whenever a MatrixSelector is evaluated, evalRange is set to the corresponding range.
	// The evaluation of the VectorSelector inside then evaluates the given range and unsets
	// the variable.
	var evalRange time.Duration
	parser.Inspect(s.Expr, func(node parser.Node, path []parser.Node) error {
		switch n := node.(type) {
		case *parser.VectorSelector:
			start, end := receiver.getTimeRangesForSelector(s, n, path, evalRange)
			if start < minTimestamp {
				minTimestamp = start
			}
			if end > maxTimestamp {
				maxTimestamp = end
			}
			evalRange = 0

		case *parser.MatrixSelector:
			evalRange = n.Range
		}
		return nil
	})

	if maxTimestamp == math.MinInt64 {
		// This happens when there was no selector. Hence no time range to select.
		minTimestamp = 0
		maxTimestamp = 0
	}

	return minTimestamp, maxTimestamp
}

func (receiver *QueryCommandRunner) getTimeRangesForSelector(s *parser.EvalStmt, n *parser.VectorSelector, path []parser.Node, evalRange time.Duration) (int64, int64) {
	start, end := timestamp.FromTime(s.Start), timestamp.FromTime(s.End)
	subqOffset, subqRange, subqTs := subqueryTimes(path)

	if subqTs != nil {
		// The timestamp on the subquery overrides the eval statement time ranges.
		start = *subqTs
		end = *subqTs
	}

	if n.Timestamp != nil {
		// The timestamp on the selector overrides everything.
		start = *n.Timestamp
		end = *n.Timestamp
	} else {
		offsetMilliseconds := durationMilliseconds(subqOffset)
		start = start - offsetMilliseconds - durationMilliseconds(subqRange)
		end = end - offsetMilliseconds
	}

	if evalRange == 0 {
		start = start - durationMilliseconds(s.LookbackDelta)
	} else {
		// For all matrix queries we want to ensure that we have (end-start) + range selected
		// this way we have `range` data before the start time
		start = start - durationMilliseconds(evalRange)
	}

	offsetMilliseconds := durationMilliseconds(n.OriginalOffset)
	start = start - offsetMilliseconds
	end = end - offsetMilliseconds

	return start, end
}

// setOffsetForAtModifier modifies the offset of vector and matrix selector
// and subquery in the tree to accommodate the timestamp of @ modifier.
// The offset is adjusted w.r.t. the given evaluation time.
func setOffsetForAtModifier(evalTime int64, expr parser.Expr) {
	getOffset := func(ts *int64, originalOffset time.Duration, path []parser.Node) time.Duration {
		if ts == nil {
			return originalOffset
		}

		subqOffset, _, subqTs := subqueryTimes(path)
		if subqTs != nil {
			subqOffset += time.Duration(evalTime-*subqTs) * time.Millisecond
		}

		offsetForTs := time.Duration(evalTime-*ts) * time.Millisecond
		offsetDiff := offsetForTs - subqOffset
		return originalOffset + offsetDiff
	}

	parser.Inspect(expr, func(node parser.Node, path []parser.Node) error {
		switch n := node.(type) {
		case *parser.VectorSelector:
			n.Offset = getOffset(n.Timestamp, n.OriginalOffset, path)

		case *parser.MatrixSelector:
			vs := n.VectorSelector.(*parser.VectorSelector)
			vs.Offset = getOffset(vs.Timestamp, vs.OriginalOffset, path)

		case *parser.SubqueryExpr:
			n.Offset = getOffset(n.Timestamp, n.OriginalOffset, path)
		}
		return nil
	})
}

func timeMilliseconds(t time.Time) int64 {
	return t.UnixNano() / int64(time.Millisecond/time.Nanosecond)
}

func durationMilliseconds(d time.Duration) int64 {
	return int64(d / (time.Millisecond / time.Nanosecond))
}

// subqueryTimes returns the sum of offsets and ranges of all subqueries in the path.
// If the @ modifier is used, then the offset and range is w.r.t. that timestamp
// (i.e. the sum is reset when we have @ modifier).
// The returned *int64 is the closest timestamp that was seen. nil for no @ modifier.
func subqueryTimes(path []parser.Node) (time.Duration, time.Duration, *int64) {
	var (
		subqOffset, subqRange time.Duration
		ts                    int64 = math.MaxInt64
	)
	for _, node := range path {
		switch n := node.(type) {
		case *parser.SubqueryExpr:
			subqOffset += n.OriginalOffset
			subqRange += n.Range
			if n.Timestamp != nil {
				// The @ modifier on subquery invalidates all the offset and
				// range till now. Hence resetting it here.
				subqOffset = n.OriginalOffset
				subqRange = n.Range
				ts = *n.Timestamp
			}
		}
	}
	var tsp *int64
	if ts != math.MaxInt64 {
		tsp = &ts
	}
	return subqOffset, subqRange, tsp
}

func newStepInvariantExpr(expr parser.Expr) parser.Expr {
	return &parser.StepInvariantExpr{Expr: expr}
}

func makeInt64Pointer(val int64) *int64 {
	valp := new(int64)
	*valp = val
	return valp
}
