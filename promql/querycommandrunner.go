package promql

import (
	"context"
	"fmt"
	influxdb "github.com/influxdata/influxdb1-client/v2"
	"github.com/pkg/errors"
	"github.com/prometheus/prometheus/model/timestamp"
	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/promql/parser"
	"github.com/wubin1989/promql2influxql/command"
	"github.com/wubin1989/promql2influxql/config"
	"math"
	"strings"
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
			MaxSamples:           cfg.MaxSamples,
			Timeout:              cfg.Timeout,
			EnableAtModifier:     cfg.PromQLConfig.EnableAtModifier,
			EnableNegativeOffset: cfg.PromQLConfig.EnableNegativeOffset,
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

func (receiver *QueryCommandRunner) Run(ctx context.Context, cmd string) (command.CommandResult, error) {
	now := time.Now()
	lines := strings.Split(cmd, command.LineBreak)
	commandResult := command.CommandResult{
		Results: make([]command.Result, 0, len(lines)),
	}
	for i, item := range lines {
		expr, err := parser.ParseExpr(item)
		if err != nil {
			var perr *parser.ParseErr
			if errors.As(err, &perr) {
				perr.LineOffset = i
				posOffset := parser.Pos(strings.Index(lines[i], item))
				perr.PositionRange.Start += posOffset
				perr.PositionRange.End += posOffset
				perr.Query = lines[i]
			}
			return command.CommandResult{}, errors.Wrap(err, "command parse fail")
		}
		executor := receiver.NewQueryExecutor(expr, now, now)
		result, err := executor.Exec(ctx)
		if err != nil {
			return command.CommandResult{}, errors.Wrap(err, "command execute fail")
		}
		commandResult.Results = append(commandResult.Results, result)
	}
	return commandResult, nil
}

func (receiver *QueryCommandRunner) Recycle() {
	receiver.Factory.Recycle(receiver)
}

func (receiver *QueryCommandRunner) NewQueryExecutor(expr parser.Expr, start, end time.Time) queryExecutor {
	return queryExecutor{
		client: receiver.Client,
		stmt: &parser.EvalStmt{
			Expr:  PreprocessExpr(expr, start, end),
			Start: start,
			End:   end,
		},
		runner: receiver,
	}
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

func (receiver *QueryCommandRunner) exec(ctx context.Context, q queryExecutor) (command.Result, error) {
	ctx, cancel := context.WithTimeout(ctx, receiver.Cfg.Timeout)
	defer cancel()

	// The base context might already be canceled on the first iteration (e.g. during shutdown).
	if err := contextDone(ctx, queryExecutionStr); err != nil {
		return command.Result{}, err
	}

	switch s := q.Statement().(type) {
	case *parser.EvalStmt:
		return receiver.execEvalStmt(ctx, q, s)
	case parser.TestStmt:
		return command.Result{}, s(ctx)
	}

	return command.Result{}, errors.Errorf("promql.QueryCommandRunner.exec: unhandled statement of type %T", q.Statement())
}

func (receiver *QueryCommandRunner) execEvalStmt(ctx context.Context, query queryExecutor, s *parser.EvalStmt) (command.Result, error) {

	//// Modify the offset of vector and matrix selectors for the @ modifier
	//// w.r.t. the start time since only 1 evaluation will be done on them.
	//setOffsetForAtModifier(timeMilliseconds(s.Start), s.Expr)
	//
	//mint, maxt := receiver.findMinMaxTime(s)
	//// Instant evaluation. This is executed as a range evaluation with one step.
	//start := timeMilliseconds(s.Start)
	//evaluator := &evaluator{
	//	startTimestamp:           start,
	//	endTimestamp:             start,
	//	interval:                 1,
	//	ctx:                      ctxInnerEval,
	//	maxSamples:               ng.maxSamplesPerQuery,
	//	logger:                   ng.logger,
	//	lookbackDelta:            s.LookbackDelta,
	//	samplesStats:             query.sampleStats,
	//	noStepSubqueryIntervalFn: ng.noStepSubqueryIntervalFn,
	//}
	//
	//val, warnings, err := evaluator.Eval(s.Expr)

	//evalSpanTimer.Finish()
	//
	//if err != nil {
	//	return nil, warnings, err
	//}
	//
	//var mat Matrix
	//
	//switch result := val.(type) {
	//case Matrix:
	//	mat = result
	//case String:
	//	return result, warnings, nil
	//default:
	//	panic(fmt.Errorf("promql.Engine.exec: invalid expression type %q", val.Type()))
	//}
	//
	//query.matrix = mat
	//switch s.Expr.Type() {
	//case parser.ValueTypeVector:
	//	// Convert matrix with one value per series into vector.
	//	vector := make(Vector, len(mat))
	//	for i, s := range mat {
	//		// Point might have a different timestamp, force it to the evaluation
	//		// timestamp as that is when we ran the evaluation.
	//		vector[i] = Sample{Metric: s.Metric, Point: Point{V: s.Points[0].V, H: s.Points[0].H, T: start}}
	//	}
	//	return vector, warnings, nil
	//case parser.ValueTypeScalar:
	//	return Scalar{V: mat[0].Points[0].V, T: start}, warnings, nil
	//case parser.ValueTypeMatrix:
	//	return mat, warnings, nil
	//default:
	//	panic(fmt.Errorf("promql.Engine.exec: unexpected expression type %q", s.Expr.Type()))
	//}

	return command.Result{}, nil
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

// PreprocessExpr wraps all possible step invariant parts of the given expression with
// StepInvariantExpr. It also resolves the preprocessors.
func PreprocessExpr(expr parser.Expr, start, end time.Time) parser.Expr {
	isStepInvariant := preprocessExprHelper(expr, start, end)
	if isStepInvariant {
		return newStepInvariantExpr(expr)
	}
	return expr
}

// preprocessExprHelper wraps the child nodes of the expression
// with a StepInvariantExpr wherever it's step invariant. The returned boolean is true if the
// passed expression qualifies to be wrapped by StepInvariantExpr.
// It also resolves the preprocessors.
func preprocessExprHelper(expr parser.Expr, start, end time.Time) bool {
	switch n := expr.(type) {
	case *parser.VectorSelector:
		if n.StartOrEnd == parser.START {
			n.Timestamp = makeInt64Pointer(timestamp.FromTime(start))
		} else if n.StartOrEnd == parser.END {
			n.Timestamp = makeInt64Pointer(timestamp.FromTime(end))
		}
		return n.Timestamp != nil

	case *parser.AggregateExpr:
		return preprocessExprHelper(n.Expr, start, end)

	case *parser.BinaryExpr:
		isInvariant1, isInvariant2 := preprocessExprHelper(n.LHS, start, end), preprocessExprHelper(n.RHS, start, end)
		if isInvariant1 && isInvariant2 {
			return true
		}

		if isInvariant1 {
			n.LHS = newStepInvariantExpr(n.LHS)
		}
		if isInvariant2 {
			n.RHS = newStepInvariantExpr(n.RHS)
		}

		return false

	case *parser.Call:
		_, ok := promql.AtModifierUnsafeFunctions[n.Func.Name]
		isStepInvariant := !ok
		isStepInvariantSlice := make([]bool, len(n.Args))
		for i := range n.Args {
			isStepInvariantSlice[i] = preprocessExprHelper(n.Args[i], start, end)
			isStepInvariant = isStepInvariant && isStepInvariantSlice[i]
		}

		if isStepInvariant {
			// The function and all arguments are step invariant.
			return true
		}

		for i, isi := range isStepInvariantSlice {
			if isi {
				n.Args[i] = newStepInvariantExpr(n.Args[i])
			}
		}
		return false

	case *parser.MatrixSelector:
		return preprocessExprHelper(n.VectorSelector, start, end)

	case *parser.SubqueryExpr:
		// Since we adjust offset for the @ modifier evaluation,
		// it gets tricky to adjust it for every subquery step.
		// Hence we wrap the inside of subquery irrespective of
		// @ on subquery (given it is also step invariant) so that
		// it is evaluated only once w.r.t. the start time of subquery.
		isInvariant := preprocessExprHelper(n.Expr, start, end)
		if isInvariant {
			n.Expr = newStepInvariantExpr(n.Expr)
		}
		if n.StartOrEnd == parser.START {
			n.Timestamp = makeInt64Pointer(timestamp.FromTime(start))
		} else if n.StartOrEnd == parser.END {
			n.Timestamp = makeInt64Pointer(timestamp.FromTime(end))
		}
		return n.Timestamp != nil

	case *parser.ParenExpr:
		return preprocessExprHelper(n.Expr, start, end)

	case *parser.UnaryExpr:
		return preprocessExprHelper(n.Expr, start, end)

	case *parser.StringLiteral, *parser.NumberLiteral:
		return true
	}

	panic(fmt.Sprintf("found unexpected node %#v", expr))
}

func newStepInvariantExpr(expr parser.Expr) parser.Expr {
	return &parser.StepInvariantExpr{Expr: expr}
}

func makeInt64Pointer(val int64) *int64 {
	valp := new(int64)
	*valp = val
	return valp
}

var _ command.IStatementExecutor = (*queryExecutor)(nil)

type queryExecutor struct {
	// Underlying data provider.
	client influxdb.Client
	// Statement of the parsed query.
	stmt parser.Statement
	// The engine against which the query is executed.
	runner *QueryCommandRunner
}

// Exec implements the Query interface.
func (q queryExecutor) Exec(ctx context.Context) (command.Result, error) {
	// Exec query.
	ret, err := q.runner.exec(ctx, q)
	if err != nil {
		return command.Result{}, errors.Wrap(err, "runner exec error")
	}
	return ret, nil
}

func (q queryExecutor) Statement() parser.Statement {
	return q.stmt
}
