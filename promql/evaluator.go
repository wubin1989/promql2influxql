package promql

//
//import (
//	"bytes"
//	"container/heap"
//	"context"
//	"fmt"
//	"github.com/prometheus/common/model"
//	"github.com/prometheus/prometheus/model/histogram"
//	"github.com/prometheus/prometheus/model/labels"
//	"github.com/prometheus/prometheus/model/value"
//	"github.com/prometheus/prometheus/promql"
//	"github.com/prometheus/prometheus/promql/parser"
//	"github.com/prometheus/prometheus/storage"
//	"github.com/prometheus/prometheus/tsdb/chunkenc"
//	"github.com/prometheus/prometheus/util/stats"
//	"go.opentelemetry.io/otel"
//	"golang.org/x/exp/slices"
//	"math"
//	"reflect"
//	"runtime"
//	"sort"
//	"strconv"
//	"time"
//)
//
//// An evaluator evaluates given expressions over given fixed timestamps. It
//// is attached to an engine through which it connects to a querier and reports
//// errors. On timeout or cancellation of its context it terminates.
//type evaluator struct {
//	ctx context.Context
//
//	startTimestamp int64 // Start time in milliseconds.
//	endTimestamp   int64 // End time in milliseconds.
//	interval       int64 // Interval in milliseconds.
//
//	maxSamples               int
//	currentSamples           int
//	lookbackDelta            time.Duration
//	samplesStats             *stats.QuerySamples
//	noStepSubqueryIntervalFn func(rangeMillis int64) int64
//}
//
//// errorf causes a panic with the input formatted into an error.
//func (ev *evaluator) errorf(format string, args ...interface{}) {
//	ev.error(fmt.Errorf(format, args...))
//}
//
//// error causes a panic with the given error.
//func (ev *evaluator) error(err error) {
//	panic(err)
//}
//
//// recover is the handler that turns panics into returns from the top level of evaluation.
//func (ev *evaluator) recover(expr parser.Expr, ws *storage.Warnings, errp *error) {
//	e := recover()
//	if e == nil {
//		return
//	}
//
//	switch err := e.(type) {
//	case runtime.Error:
//		// Print the stack trace but do not inhibit the running application.
//		buf := make([]byte, 64<<10)
//		buf = buf[:runtime.Stack(buf, false)]
//
//		*errp = fmt.Errorf("unexpected error: %w", err)
//	case error:
//		*errp = err
//	default:
//		*errp = fmt.Errorf("%v", err)
//	}
//}
//
//func (ev *evaluator) Eval(expr parser.Expr) (v parser.Value, ws storage.Warnings, err error) {
//	defer ev.recover(expr, &ws, &err)
//
//	v, ws = ev.eval(expr)
//	return v, ws, nil
//}
//
//// rangeEval evaluates the given expressions, and then for each step calls
//// the given funcCall with the values computed for each expression at that
//// step. The return value is the combination into time series of all the
//// function call results.
//// The prepSeries function (if provided) can be used to prepare the helper
//// for each series, then passed to each call funcCall.
//func (ev *evaluator) rangeEval(prepSeries func(labels.Labels, *promql.EvalSeriesHelper), funcCall func([]parser.Value, [][]promql.EvalSeriesHelper, *promql.EvalNodeHelper) (promql.Vector, storage.Warnings), exprs ...parser.Expr) (promql.Matrix, storage.Warnings) {
//	numSteps := int((ev.endTimestamp-ev.startTimestamp)/ev.interval) + 1
//	matrixes := make([]promql.Matrix, len(exprs))
//	origMatrixes := make([]promql.Matrix, len(exprs))
//	originalNumSamples := ev.currentSamples
//
//	var warnings storage.Warnings
//	for i, e := range exprs {
//		// Functions will take string arguments from the expressions, not the values.
//		if e != nil && e.Type() != parser.ValueTypeString {
//			// ev.currentSamples will be updated to the correct value within the ev.eval call.
//			val, ws := ev.eval(e)
//			warnings = append(warnings, ws...)
//			matrixes[i] = val.(promql.Matrix)
//
//			// Keep a copy of the original point slices so that they
//			// can be returned to the pool.
//			origMatrixes[i] = make(promql.Matrix, len(matrixes[i]))
//			copy(origMatrixes[i], matrixes[i])
//		}
//	}
//
//	vectors := make([]promql.Vector, len(exprs)) // Input vectors for the function.
//	args := make([]parser.Value, len(exprs))     // Argument to function.
//	// Create an output vector that is as big as the input matrix with
//	// the most time series.
//	biggestLen := 1
//	for i := range exprs {
//		vectors[i] = make(promql.Vector, 0, len(matrixes[i]))
//		if len(matrixes[i]) > biggestLen {
//			biggestLen = len(matrixes[i])
//		}
//	}
//	enh := &promql.EvalNodeHelper{Out: make(promql.Vector, 0, biggestLen)}
//	seriess := make(map[uint64]promql.Series, biggestLen) // Output series by series hash.
//	tempNumSamples := ev.currentSamples
//
//	var (
//		seriesHelpers [][]promql.EvalSeriesHelper
//		bufHelpers    [][]promql.EvalSeriesHelper // Buffer updated on each step
//	)
//
//	// If the series preparation function is provided, we should run it for
//	// every single series in the matrix.
//	if prepSeries != nil {
//		seriesHelpers = make([][]promql.EvalSeriesHelper, len(exprs))
//		bufHelpers = make([][]promql.EvalSeriesHelper, len(exprs))
//
//		for i := range exprs {
//			seriesHelpers[i] = make([]promql.EvalSeriesHelper, len(matrixes[i]))
//			bufHelpers[i] = make([]promql.EvalSeriesHelper, len(matrixes[i]))
//
//			for si, series := range matrixes[i] {
//				h := seriesHelpers[i][si]
//				prepSeries(series.Metric, &h)
//				seriesHelpers[i][si] = h
//			}
//		}
//	}
//
//	for ts := ev.startTimestamp; ts <= ev.endTimestamp; ts += ev.interval {
//		if err := contextDone(ev.ctx, "expression evaluation"); err != nil {
//			ev.error(err)
//		}
//		// Reset number of samples in memory after each timestamp.
//		ev.currentSamples = tempNumSamples
//		// Gather input vectors for this timestamp.
//		for i := range exprs {
//			vectors[i] = vectors[i][:0]
//
//			if prepSeries != nil {
//				bufHelpers[i] = bufHelpers[i][:0]
//			}
//
//			for si, series := range matrixes[i] {
//				for _, point := range series.Points {
//					if point.T == ts {
//						if ev.currentSamples < ev.maxSamples {
//							vectors[i] = append(vectors[i], promql.Sample{Metric: series.Metric, Point: point})
//							if prepSeries != nil {
//								bufHelpers[i] = append(bufHelpers[i], seriesHelpers[i][si])
//							}
//
//							// Move input vectors forward so we don't have to re-scan the same
//							// past points at the next step.
//							matrixes[i][si].Points = series.Points[1:]
//							ev.currentSamples++
//						} else {
//							ev.error(promql.ErrTooManySamples(queryExecutionStr))
//						}
//					}
//					break
//				}
//			}
//			args[i] = vectors[i]
//			ev.samplesStats.UpdatePeak(ev.currentSamples)
//		}
//
//		// Make the function call.
//		enh.Ts = ts
//		result, ws := funcCall(args, bufHelpers, enh)
//		if result.ContainsSameLabelset() {
//			ev.errorf("vector cannot contain metrics with the same labelset")
//		}
//		enh.Out = result[:0] // Reuse result vector.
//		warnings = append(warnings, ws...)
//
//		ev.currentSamples += len(result)
//		// When we reset currentSamples to tempNumSamples during the next iteration of the loop it also
//		// needs to include the samples from the result here, as they're still in memory.
//		tempNumSamples += len(result)
//		ev.samplesStats.UpdatePeak(ev.currentSamples)
//
//		if ev.currentSamples > ev.maxSamples {
//			ev.error(promql.ErrTooManySamples(queryExecutionStr))
//		}
//		ev.samplesStats.UpdatePeak(ev.currentSamples)
//
//		// If this could be an instant query, shortcut so as not to change sort order.
//		if ev.endTimestamp == ev.startTimestamp {
//			mat := make(promql.Matrix, len(result))
//			for i, s := range result {
//				s.Point.T = ts
//				mat[i] = promql.Series{Metric: s.Metric, Points: []promql.Point{s.Point}}
//			}
//			ev.currentSamples = originalNumSamples + mat.TotalSamples()
//			ev.samplesStats.UpdatePeak(ev.currentSamples)
//			return mat, warnings
//		}
//
//		// Add samples in output vector to output series.
//		for _, sample := range result {
//			h := sample.Metric.Hash()
//			ss, ok := seriess[h]
//			if !ok {
//				ss = promql.Series{
//					Metric: sample.Metric,
//					Points: getPointSlice(numSteps),
//				}
//			}
//			sample.Point.T = ts
//			ss.Points = append(ss.Points, sample.Point)
//			seriess[h] = ss
//
//		}
//	}
//
//	// Reuse the original point slices.
//	for _, m := range origMatrixes {
//		for _, s := range m {
//			putPointSlice(s.Points)
//		}
//	}
//	// Assemble the output matrix. By the time we get here we know we don't have too many samples.
//	mat := make(Matrix, 0, len(seriess))
//	for _, ss := range seriess {
//		mat = append(mat, ss)
//	}
//	ev.currentSamples = originalNumSamples + mat.TotalSamples()
//	ev.samplesStats.UpdatePeak(ev.currentSamples)
//	return mat, warnings
//}
//
//// evalSubquery evaluates given SubqueryExpr and returns an equivalent
//// evaluated MatrixSelector in its place. Note that the Name and LabelMatchers are not set.
//func (ev *evaluator) evalSubquery(subq *parser.SubqueryExpr) (*parser.MatrixSelector, int, storage.Warnings) {
//	samplesStats := ev.samplesStats
//	// Avoid double counting samples when running a subquery, those samples will be counted in later stage.
//	ev.samplesStats = ev.samplesStats.NewChild()
//	val, ws := ev.eval(subq)
//	// But do incorporate the peak from the subquery
//	samplesStats.UpdatePeakFromSubquery(ev.samplesStats)
//	ev.samplesStats = samplesStats
//	mat := val.(promql.Matrix)
//	vs := &parser.VectorSelector{
//		OriginalOffset: subq.OriginalOffset,
//		Offset:         subq.Offset,
//		Series:         make([]storage.Series, 0, len(mat)),
//		Timestamp:      subq.Timestamp,
//	}
//	if subq.Timestamp != nil {
//		// The offset of subquery is not modified in case of @ modifier.
//		// Hence we take care of that here for the result.
//		vs.Offset = subq.OriginalOffset + time.Duration(ev.startTimestamp-*subq.Timestamp)*time.Millisecond
//	}
//	ms := &parser.MatrixSelector{
//		Range:          subq.Range,
//		VectorSelector: vs,
//	}
//	totalSamples := 0
//	for _, s := range mat {
//		totalSamples += len(s.Points)
//		vs.Series = append(vs.Series, NewStorageSeries(s))
//	}
//	return ms, totalSamples, ws
//}
//
//// eval evaluates the given expression as the given AST expression node requires.
//func (ev *evaluator) eval(expr parser.Expr) (parser.Value, storage.Warnings) {
//	// This is the top-level evaluation method.
//	// Thus, we check for timeout/cancellation here.
//	if err := contextDone(ev.ctx, "expression evaluation"); err != nil {
//		ev.error(err)
//	}
//	numSteps := int((ev.endTimestamp-ev.startTimestamp)/ev.interval) + 1
//
//	// Create a new span to help investigate inner evaluation performances.
//	ctxWithSpan, span := otel.Tracer("").Start(ev.ctx, stats.InnerEvalTime.SpanOperation()+" eval "+reflect.TypeOf(expr).String())
//	ev.ctx = ctxWithSpan
//	defer span.End()
//
//	switch e := expr.(type) {
//	case *parser.AggregateExpr:
//		// Grouping labels must be sorted (expected both by generateGroupingKey() and aggregation()).
//		sortedGrouping := e.Grouping
//		slices.Sort(sortedGrouping)
//
//		// Prepare a function to initialise series helpers with the grouping key.
//		buf := make([]byte, 0, 1024)
//		initSeries := func(series labels.Labels, h *EvalSeriesHelper) {
//			h.groupingKey, buf = generateGroupingKey(series, sortedGrouping, e.Without, buf)
//		}
//
//		unwrapParenExpr(&e.Param)
//		param := unwrapStepInvariantExpr(e.Param)
//		unwrapParenExpr(&param)
//		if s, ok := param.(*parser.StringLiteral); ok {
//			return ev.rangeEval(initSeries, func(v []parser.Value, sh [][]EvalSeriesHelper, enh *EvalNodeHelper) (Vector, storage.Warnings) {
//				return ev.aggregation(e.Op, sortedGrouping, e.Without, s.Val, v[0].(Vector), sh[0], enh), nil
//			}, e.Expr)
//		}
//
//		return ev.rangeEval(initSeries, func(v []parser.Value, sh [][]EvalSeriesHelper, enh *EvalNodeHelper) (Vector, storage.Warnings) {
//			var param float64
//			if e.Param != nil {
//				param = v[0].(Vector)[0].V
//			}
//			return ev.aggregation(e.Op, sortedGrouping, e.Without, param, v[1].(Vector), sh[1], enh), nil
//		}, e.Param, e.Expr)
//
//	case *parser.Call:
//		call := FunctionCalls[e.Func.Name]
//		if e.Func.Name == "timestamp" {
//			// Matrix evaluation always returns the evaluation time,
//			// so this function needs special handling when given
//			// a vector selector.
//			unwrapParenExpr(&e.Args[0])
//			arg := unwrapStepInvariantExpr(e.Args[0])
//			unwrapParenExpr(&arg)
//			vs, ok := arg.(*parser.VectorSelector)
//			if ok {
//				return ev.rangeEval(nil, func(v []parser.Value, _ [][]EvalSeriesHelper, enh *EvalNodeHelper) (Vector, storage.Warnings) {
//					if vs.Timestamp != nil {
//						// This is a special case only for "timestamp" since the offset
//						// needs to be adjusted for every point.
//						vs.Offset = time.Duration(enh.Ts-*vs.Timestamp) * time.Millisecond
//					}
//					val, ws := ev.vectorSelector(vs, enh.Ts)
//					return call([]parser.Value{val}, e.Args, enh), ws
//				})
//			}
//		}
//
//		// Check if the function has a matrix argument.
//		var (
//			matrixArgIndex int
//			matrixArg      bool
//			warnings       storage.Warnings
//		)
//		for i := range e.Args {
//			unwrapParenExpr(&e.Args[i])
//			a := unwrapStepInvariantExpr(e.Args[i])
//			unwrapParenExpr(&a)
//			if _, ok := a.(*parser.MatrixSelector); ok {
//				matrixArgIndex = i
//				matrixArg = true
//				break
//			}
//			// parser.SubqueryExpr can be used in place of parser.MatrixSelector.
//			if subq, ok := a.(*parser.SubqueryExpr); ok {
//				matrixArgIndex = i
//				matrixArg = true
//				// Replacing parser.SubqueryExpr with parser.MatrixSelector.
//				val, totalSamples, ws := ev.evalSubquery(subq)
//				e.Args[i] = val
//				warnings = append(warnings, ws...)
//				defer func() {
//					// subquery result takes space in the memory. Get rid of that at the end.
//					val.VectorSelector.(*parser.VectorSelector).Series = nil
//					ev.currentSamples -= totalSamples
//				}()
//				break
//			}
//		}
//		if !matrixArg {
//			// Does not have a matrix argument.
//			return ev.rangeEval(nil, func(v []parser.Value, _ [][]EvalSeriesHelper, enh *EvalNodeHelper) (Vector, storage.Warnings) {
//				return call(v, e.Args, enh), warnings
//			}, e.Args...)
//		}
//
//		inArgs := make([]parser.Value, len(e.Args))
//		// Evaluate any non-matrix arguments.
//		otherArgs := make([]Matrix, len(e.Args))
//		otherInArgs := make([]Vector, len(e.Args))
//		for i, e := range e.Args {
//			if i != matrixArgIndex {
//				val, ws := ev.eval(e)
//				otherArgs[i] = val.(Matrix)
//				otherInArgs[i] = Vector{Sample{}}
//				inArgs[i] = otherInArgs[i]
//				warnings = append(warnings, ws...)
//			}
//		}
//
//		unwrapParenExpr(&e.Args[matrixArgIndex])
//		arg := unwrapStepInvariantExpr(e.Args[matrixArgIndex])
//		unwrapParenExpr(&arg)
//		sel := arg.(*parser.MatrixSelector)
//		selVS := sel.VectorSelector.(*parser.VectorSelector)
//
//		ws, err := checkAndExpandSeriesSet(ev.ctx, sel)
//		warnings = append(warnings, ws...)
//		if err != nil {
//			ev.error(errWithWarnings{fmt.Errorf("expanding series: %w", err), warnings})
//		}
//		mat := make(Matrix, 0, len(selVS.Series)) // Output matrix.
//		offset := durationMilliseconds(selVS.Offset)
//		selRange := durationMilliseconds(sel.Range)
//		stepRange := selRange
//		if stepRange > ev.interval {
//			stepRange = ev.interval
//		}
//		// Reuse objects across steps to save memory allocations.
//		points := getPointSlice(16)
//		inMatrix := make(Matrix, 1)
//		inArgs[matrixArgIndex] = inMatrix
//		enh := &EvalNodeHelper{Out: make(Vector, 0, 1)}
//		// Process all the calls for one time series at a time.
//		it := storage.NewBuffer(selRange)
//		var chkIter chunkenc.Iterator
//		for i, s := range selVS.Series {
//			ev.currentSamples -= len(points)
//			points = points[:0]
//			chkIter = s.Iterator(chkIter)
//			it.Reset(chkIter)
//			metric := selVS.Series[i].Labels()
//			// The last_over_time function acts like offset; thus, it
//			// should keep the metric name.  For all the other range
//			// vector functions, the only change needed is to drop the
//			// metric name in the output.
//			if e.Func.Name != "last_over_time" {
//				metric = dropMetricName(metric)
//			}
//			ss := Series{
//				Metric: metric,
//				Points: getPointSlice(numSteps),
//			}
//			inMatrix[0].Metric = selVS.Series[i].Labels()
//			for ts, step := ev.startTimestamp, -1; ts <= ev.endTimestamp; ts += ev.interval {
//				step++
//				// Set the non-matrix arguments.
//				// They are scalar, so it is safe to use the step number
//				// when looking up the argument, as there will be no gaps.
//				for j := range e.Args {
//					if j != matrixArgIndex {
//						otherInArgs[j][0].V = otherArgs[j][0].Points[step].V
//					}
//				}
//				maxt := ts - offset
//				mint := maxt - selRange
//				// Evaluate the matrix selector for this series for this step.
//				points = ev.matrixIterSlice(it, mint, maxt, points)
//				if len(points) == 0 {
//					continue
//				}
//				inMatrix[0].Points = points
//				enh.Ts = ts
//				// Make the function call.
//				outVec := call(inArgs, e.Args, enh)
//				ev.samplesStats.IncrementSamplesAtStep(step, int64(len(points)))
//				enh.Out = outVec[:0]
//				if len(outVec) > 0 {
//					ss.Points = append(ss.Points, Point{V: outVec[0].Point.V, H: outVec[0].Point.H, T: ts})
//				}
//				// Only buffer stepRange milliseconds from the second step on.
//				it.ReduceDelta(stepRange)
//			}
//			if len(ss.Points) > 0 {
//				if ev.currentSamples+len(ss.Points) <= ev.maxSamples {
//					mat = append(mat, ss)
//					ev.currentSamples += len(ss.Points)
//				} else {
//					ev.error(ErrTooManySamples(env))
//				}
//			} else {
//				putPointSlice(ss.Points)
//			}
//			ev.samplesStats.UpdatePeak(ev.currentSamples)
//		}
//		ev.samplesStats.UpdatePeak(ev.currentSamples)
//
//		ev.currentSamples -= len(points)
//		putPointSlice(points)
//
//		// The absent_over_time function returns 0 or 1 series. So far, the matrix
//		// contains multiple series. The following code will create a new series
//		// with values of 1 for the timestamps where no series has value.
//		if e.Func.Name == "absent_over_time" {
//			steps := int(1 + (ev.endTimestamp-ev.startTimestamp)/ev.interval)
//			// Iterate once to look for a complete series.
//			for _, s := range mat {
//				if len(s.Points) == steps {
//					return Matrix{}, warnings
//				}
//			}
//
//			found := map[int64]struct{}{}
//
//			for i, s := range mat {
//				for _, p := range s.Points {
//					found[p.T] = struct{}{}
//				}
//				if i > 0 && len(found) == steps {
//					return Matrix{}, warnings
//				}
//			}
//
//			newp := make([]Point, 0, steps-len(found))
//			for ts := ev.startTimestamp; ts <= ev.endTimestamp; ts += ev.interval {
//				if _, ok := found[ts]; !ok {
//					newp = append(newp, Point{T: ts, V: 1})
//				}
//			}
//
//			return Matrix{
//				Series{
//					Metric: createLabelsForAbsentFunction(e.Args[0]),
//					Points: newp,
//				},
//			}, warnings
//		}
//
//		if mat.ContainsSameLabelset() {
//			ev.errorf("vector cannot contain metrics with the same labelset")
//		}
//
//		return mat, warnings
//
//	case *parser.ParenExpr:
//		return ev.eval(e.Expr)
//
//	case *parser.UnaryExpr:
//		val, ws := ev.eval(e.Expr)
//		mat := val.(Matrix)
//		if e.Op == parser.SUB {
//			for i := range mat {
//				mat[i].Metric = dropMetricName(mat[i].Metric)
//				for j := range mat[i].Points {
//					mat[i].Points[j].V = -mat[i].Points[j].V
//				}
//			}
//			if mat.ContainsSameLabelset() {
//				ev.errorf("vector cannot contain metrics with the same labelset")
//			}
//		}
//		return mat, ws
//
//	case *parser.BinaryExpr:
//		switch lt, rt := e.LHS.Type(), e.RHS.Type(); {
//		case lt == parser.ValueTypeScalar && rt == parser.ValueTypeScalar:
//			return ev.rangeEval(nil, func(v []parser.Value, _ [][]EvalSeriesHelper, enh *EvalNodeHelper) (Vector, storage.Warnings) {
//				val := scalarBinop(e.Op, v[0].(Vector)[0].Point.V, v[1].(Vector)[0].Point.V)
//				return append(enh.Out, Sample{Point: Point{V: val}}), nil
//			}, e.LHS, e.RHS)
//		case lt == parser.ValueTypeVector && rt == parser.ValueTypeVector:
//			// Function to compute the join signature for each series.
//			buf := make([]byte, 0, 1024)
//			sigf := signatureFunc(e.VectorMatching.On, buf, e.VectorMatching.MatchingLabels...)
//			initSignatures := func(series labels.Labels, h *EvalSeriesHelper) {
//				h.signature = sigf(series)
//			}
//			switch e.Op {
//			case parser.LAND:
//				return ev.rangeEval(initSignatures, func(v []parser.Value, sh [][]EvalSeriesHelper, enh *EvalNodeHelper) (Vector, storage.Warnings) {
//					return ev.VectorAnd(v[0].(Vector), v[1].(Vector), e.VectorMatching, sh[0], sh[1], enh), nil
//				}, e.LHS, e.RHS)
//			case parser.LOR:
//				return ev.rangeEval(initSignatures, func(v []parser.Value, sh [][]EvalSeriesHelper, enh *EvalNodeHelper) (Vector, storage.Warnings) {
//					return ev.VectorOr(v[0].(Vector), v[1].(Vector), e.VectorMatching, sh[0], sh[1], enh), nil
//				}, e.LHS, e.RHS)
//			case parser.LUNLESS:
//				return ev.rangeEval(initSignatures, func(v []parser.Value, sh [][]EvalSeriesHelper, enh *EvalNodeHelper) (Vector, storage.Warnings) {
//					return ev.VectorUnless(v[0].(Vector), v[1].(Vector), e.VectorMatching, sh[0], sh[1], enh), nil
//				}, e.LHS, e.RHS)
//			default:
//				return ev.rangeEval(initSignatures, func(v []parser.Value, sh [][]EvalSeriesHelper, enh *EvalNodeHelper) (Vector, storage.Warnings) {
//					return ev.VectorBinop(e.Op, v[0].(Vector), v[1].(Vector), e.VectorMatching, e.ReturnBool, sh[0], sh[1], enh), nil
//				}, e.LHS, e.RHS)
//			}
//
//		case lt == parser.ValueTypeVector && rt == parser.ValueTypeScalar:
//			return ev.rangeEval(nil, func(v []parser.Value, _ [][]EvalSeriesHelper, enh *EvalNodeHelper) (Vector, storage.Warnings) {
//				return ev.VectorscalarBinop(e.Op, v[0].(Vector), Scalar{V: v[1].(Vector)[0].Point.V}, false, e.ReturnBool, enh), nil
//			}, e.LHS, e.RHS)
//
//		case lt == parser.ValueTypeScalar && rt == parser.ValueTypeVector:
//			return ev.rangeEval(nil, func(v []parser.Value, _ [][]EvalSeriesHelper, enh *EvalNodeHelper) (Vector, storage.Warnings) {
//				return ev.VectorscalarBinop(e.Op, v[1].(Vector), Scalar{V: v[0].(Vector)[0].Point.V}, true, e.ReturnBool, enh), nil
//			}, e.LHS, e.RHS)
//		}
//
//	case *parser.NumberLiteral:
//		return ev.rangeEval(nil, func(v []parser.Value, _ [][]EvalSeriesHelper, enh *EvalNodeHelper) (Vector, storage.Warnings) {
//			return append(enh.Out, Sample{Point: Point{V: e.Val}, Metric: labels.EmptyLabels()}), nil
//		})
//
//	case *parser.StringLiteral:
//		return String{V: e.Val, T: ev.startTimestamp}, nil
//
//	case *parser.VectorSelector:
//		ws, err := checkAndExpandSeriesSet(ev.ctx, e)
//		if err != nil {
//			ev.error(errWithWarnings{fmt.Errorf("expanding series: %w", err), ws})
//		}
//		mat := make(Matrix, 0, len(e.Series))
//		it := storage.NewMemoizedEmptyIterator(durationMilliseconds(ev.lookbackDelta))
//		var chkIter chunkenc.Iterator
//		for i, s := range e.Series {
//			chkIter = s.Iterator(chkIter)
//			it.Reset(chkIter)
//			ss := Series{
//				Metric: e.Series[i].Labels(),
//				Points: getPointSlice(numSteps),
//			}
//
//			for ts, step := ev.startTimestamp, -1; ts <= ev.endTimestamp; ts += ev.interval {
//				step++
//				_, v, h, ok := ev.vectorSelectorSingle(it, e, ts)
//				if ok {
//					if ev.currentSamples < ev.maxSamples {
//						ss.Points = append(ss.Points, Point{V: v, H: h, T: ts})
//						ev.samplesStats.IncrementSamplesAtStep(step, 1)
//						ev.currentSamples++
//					} else {
//						ev.error(ErrTooManySamples(env))
//					}
//				}
//			}
//
//			if len(ss.Points) > 0 {
//				mat = append(mat, ss)
//			} else {
//				putPointSlice(ss.Points)
//			}
//		}
//		ev.samplesStats.UpdatePeak(ev.currentSamples)
//		return mat, ws
//
//	case *parser.MatrixSelector:
//		if ev.startTimestamp != ev.endTimestamp {
//			panic(errors.New("cannot do range evaluation of matrix selector"))
//		}
//		return ev.matrixSelector(e)
//
//	case *parser.SubqueryExpr:
//		offsetMillis := durationMilliseconds(e.Offset)
//		rangeMillis := durationMilliseconds(e.Range)
//		newEv := &evaluator{
//			endTimestamp:             ev.endTimestamp - offsetMillis,
//			ctx:                      ev.ctx,
//			currentSamples:           ev.currentSamples,
//			maxSamples:               ev.maxSamples,
//			logger:                   ev.logger,
//			lookbackDelta:            ev.lookbackDelta,
//			samplesStats:             ev.samplesStats.NewChild(),
//			noStepSubqueryIntervalFn: ev.noStepSubqueryIntervalFn,
//		}
//
//		if e.Step != 0 {
//			newEv.interval = durationMilliseconds(e.Step)
//		} else {
//			newEv.interval = ev.noStepSubqueryIntervalFn(rangeMillis)
//		}
//
//		// Start with the first timestamp after (ev.startTimestamp - offset - range)
//		// that is aligned with the step (multiple of 'newEv.interval').
//		newEv.startTimestamp = newEv.interval * ((ev.startTimestamp - offsetMillis - rangeMillis) / newEv.interval)
//		if newEv.startTimestamp < (ev.startTimestamp - offsetMillis - rangeMillis) {
//			newEv.startTimestamp += newEv.interval
//		}
//
//		if newEv.startTimestamp != ev.startTimestamp {
//			// Adjust the offset of selectors based on the new
//			// start time of the evaluator since the calculation
//			// of the offset with @ happens w.r.t. the start time.
//			setOffsetForAtModifier(newEv.startTimestamp, e.Expr)
//		}
//
//		res, ws := newEv.eval(e.Expr)
//		ev.currentSamples = newEv.currentSamples
//		ev.samplesStats.UpdatePeakFromSubquery(newEv.samplesStats)
//		ev.samplesStats.IncrementSamplesAtTimestamp(ev.endTimestamp, newEv.samplesStats.TotalSamples)
//		return res, ws
//	case *parser.StepInvariantExpr:
//		switch ce := e.Expr.(type) {
//		case *parser.StringLiteral, *parser.NumberLiteral:
//			return ev.eval(ce)
//		}
//
//		newEv := &evaluator{
//			startTimestamp:           ev.startTimestamp,
//			endTimestamp:             ev.startTimestamp, // Always a single evaluation.
//			interval:                 ev.interval,
//			ctx:                      ev.ctx,
//			currentSamples:           ev.currentSamples,
//			maxSamples:               ev.maxSamples,
//			logger:                   ev.logger,
//			lookbackDelta:            ev.lookbackDelta,
//			samplesStats:             ev.samplesStats.NewChild(),
//			noStepSubqueryIntervalFn: ev.noStepSubqueryIntervalFn,
//		}
//		res, ws := newEv.eval(e.Expr)
//		ev.currentSamples = newEv.currentSamples
//		ev.samplesStats.UpdatePeakFromSubquery(newEv.samplesStats)
//		for ts, step := ev.startTimestamp, -1; ts <= ev.endTimestamp; ts = ts + ev.interval {
//			step++
//			ev.samplesStats.IncrementSamplesAtStep(step, newEv.samplesStats.TotalSamples)
//		}
//		switch e.Expr.(type) {
//		case *parser.MatrixSelector, *parser.SubqueryExpr:
//			// We do not duplicate results for range selectors since result is a matrix
//			// with their unique timestamps which does not depend on the step.
//			return res, ws
//		}
//
//		// For every evaluation while the value remains same, the timestamp for that
//		// value would change for different eval times. Hence we duplicate the result
//		// with changed timestamps.
//		mat, ok := res.(Matrix)
//		if !ok {
//			panic(fmt.Errorf("unexpected result in StepInvariantExpr evaluation: %T", expr))
//		}
//		for i := range mat {
//			if len(mat[i].Points) != 1 {
//				panic(fmt.Errorf("unexpected number of samples"))
//			}
//			for ts := ev.startTimestamp + ev.interval; ts <= ev.endTimestamp; ts = ts + ev.interval {
//				mat[i].Points = append(mat[i].Points, Point{
//					T: ts,
//					V: mat[i].Points[0].V,
//					H: mat[i].Points[0].H,
//				})
//				ev.currentSamples++
//				if ev.currentSamples > ev.maxSamples {
//					ev.error(ErrTooManySamples(env))
//				}
//			}
//		}
//		ev.samplesStats.UpdatePeak(ev.currentSamples)
//		return res, ws
//	}
//
//	panic(fmt.Errorf("unhandled expression of type: %T", expr))
//}
//
//// vectorSelector evaluates a *parser.VectorSelector expression.
//func (ev *evaluator) vectorSelector(node *parser.VectorSelector, ts int64) (Vector, storage.Warnings) {
//	ws, err := checkAndExpandSeriesSet(ev.ctx, node)
//	if err != nil {
//		ev.error(errWithWarnings{fmt.Errorf("expanding series: %w", err), ws})
//	}
//	vec := make(Vector, 0, len(node.Series))
//	it := storage.NewMemoizedEmptyIterator(durationMilliseconds(ev.lookbackDelta))
//	var chkIter chunkenc.Iterator
//	for i, s := range node.Series {
//		chkIter = s.Iterator(chkIter)
//		it.Reset(chkIter)
//
//		t, v, h, ok := ev.vectorSelectorSingle(it, node, ts)
//		if ok {
//			vec = append(vec, Sample{
//				Metric: node.Series[i].Labels(),
//				Point:  Point{V: v, H: h, T: t},
//			})
//
//			ev.currentSamples++
//			ev.samplesStats.IncrementSamplesAtTimestamp(ts, 1)
//			if ev.currentSamples > ev.maxSamples {
//				ev.error(ErrTooManySamples(env))
//			}
//		}
//
//	}
//	ev.samplesStats.UpdatePeak(ev.currentSamples)
//	return vec, ws
//}
//
//// vectorSelectorSingle evaluates an instant vector for the iterator of one time series.
//func (ev *evaluator) vectorSelectorSingle(it *storage.MemoizedSeriesIterator, node *parser.VectorSelector, ts int64) (
//	int64, float64, *histogram.FloatHistogram, bool,
//) {
//	refTime := ts - durationMilliseconds(node.Offset)
//	var t int64
//	var v float64
//	var h *histogram.FloatHistogram
//
//	valueType := it.Seek(refTime)
//	switch valueType {
//	case chunkenc.ValNone:
//		if it.Err() != nil {
//			ev.error(it.Err())
//		}
//	case chunkenc.ValFloat:
//		t, v = it.At()
//	case chunkenc.ValHistogram, chunkenc.ValFloatHistogram:
//		t, h = it.AtFloatHistogram()
//	default:
//		panic(fmt.Errorf("unknown value type %v", valueType))
//	}
//	if valueType == chunkenc.ValNone || t > refTime {
//		var ok bool
//		t, v, _, h, ok = it.PeekPrev()
//		if !ok || t < refTime-durationMilliseconds(ev.lookbackDelta) {
//			return 0, 0, nil, false
//		}
//	}
//	if value.IsStaleNaN(v) || (h != nil && value.IsStaleNaN(h.Sum)) {
//		return 0, 0, nil, false
//	}
//	return t, v, h, true
//}
//
//// matrixSelector evaluates a *parser.MatrixSelector expression.
//func (ev *evaluator) matrixSelector(node *parser.MatrixSelector) (Matrix, storage.Warnings) {
//	var (
//		vs = node.VectorSelector.(*parser.VectorSelector)
//
//		offset = durationMilliseconds(vs.Offset)
//		maxt   = ev.startTimestamp - offset
//		mint   = maxt - durationMilliseconds(node.Range)
//		matrix = make(Matrix, 0, len(vs.Series))
//
//		it = storage.NewBuffer(durationMilliseconds(node.Range))
//	)
//	ws, err := checkAndExpandSeriesSet(ev.ctx, node)
//	if err != nil {
//		ev.error(errWithWarnings{fmt.Errorf("expanding series: %w", err), ws})
//	}
//
//	var chkIter chunkenc.Iterator
//	series := vs.Series
//	for i, s := range series {
//		if err := contextDone(ev.ctx, "expression evaluation"); err != nil {
//			ev.error(err)
//		}
//		chkIter = s.Iterator(chkIter)
//		it.Reset(chkIter)
//		ss := Series{
//			Metric: series[i].Labels(),
//		}
//
//		ss.Points = ev.matrixIterSlice(it, mint, maxt, getPointSlice(16))
//		ev.samplesStats.IncrementSamplesAtTimestamp(ev.startTimestamp, int64(len(ss.Points)))
//
//		if len(ss.Points) > 0 {
//			matrix = append(matrix, ss)
//		} else {
//			putPointSlice(ss.Points)
//		}
//	}
//	return matrix, ws
//}
//
//// matrixIterSlice populates a matrix vector covering the requested range for a
//// single time series, with points retrieved from an iterator.
////
//// As an optimization, the matrix vector may already contain points of the same
//// time series from the evaluation of an earlier step (with lower mint and maxt
//// values). Any such points falling before mint are discarded; points that fall
//// into the [mint, maxt] range are retained; only points with later timestamps
//// are populated from the iterator.
//func (ev *evaluator) matrixIterSlice(it *storage.BufferedSeriesIterator, mint, maxt int64, out []Point) []Point {
//	if len(out) > 0 && out[len(out)-1].T >= mint {
//		// There is an overlap between previous and current ranges, retain common
//		// points. In most such cases:
//		//   (a) the overlap is significantly larger than the eval step; and/or
//		//   (b) the number of samples is relatively small.
//		// so a linear search will be as fast as a binary search.
//		var drop int
//		for drop = 0; out[drop].T < mint; drop++ {
//		}
//		ev.currentSamples -= drop
//		copy(out, out[drop:])
//		out = out[:len(out)-drop]
//		// Only append points with timestamps after the last timestamp we have.
//		mint = out[len(out)-1].T + 1
//	} else {
//		ev.currentSamples -= len(out)
//		out = out[:0]
//	}
//
//	soughtValueType := it.Seek(maxt)
//	if soughtValueType == chunkenc.ValNone {
//		if it.Err() != nil {
//			ev.error(it.Err())
//		}
//	}
//
//	buf := it.Buffer()
//loop:
//	for {
//		switch buf.Next() {
//		case chunkenc.ValNone:
//			break loop
//		case chunkenc.ValFloatHistogram, chunkenc.ValHistogram:
//			t, h := buf.AtFloatHistogram()
//			if value.IsStaleNaN(h.Sum) {
//				continue loop
//			}
//			// Values in the buffer are guaranteed to be smaller than maxt.
//			if t >= mint {
//				if ev.currentSamples >= ev.maxSamples {
//					ev.error(ErrTooManySamples(env))
//				}
//				ev.currentSamples++
//				out = append(out, Point{T: t, H: h})
//			}
//		case chunkenc.ValFloat:
//			t, v := buf.At()
//			if value.IsStaleNaN(v) {
//				continue loop
//			}
//			// Values in the buffer are guaranteed to be smaller than maxt.
//			if t >= mint {
//				if ev.currentSamples >= ev.maxSamples {
//					ev.error(ErrTooManySamples(env))
//				}
//				ev.currentSamples++
//				out = append(out, Point{T: t, V: v})
//			}
//		}
//	}
//	// The sought sample might also be in the range.
//	switch soughtValueType {
//	case chunkenc.ValFloatHistogram, chunkenc.ValHistogram:
//		t, h := it.AtFloatHistogram()
//		if t == maxt && !value.IsStaleNaN(h.Sum) {
//			if ev.currentSamples >= ev.maxSamples {
//				ev.error(ErrTooManySamples(env))
//			}
//			out = append(out, Point{T: t, H: h})
//			ev.currentSamples++
//		}
//	case chunkenc.ValFloat:
//		t, v := it.At()
//		if t == maxt && !value.IsStaleNaN(v) {
//			if ev.currentSamples >= ev.maxSamples {
//				ev.error(ErrTooManySamples(env))
//			}
//			out = append(out, Point{T: t, V: v})
//			ev.currentSamples++
//		}
//	}
//	ev.samplesStats.UpdatePeak(ev.currentSamples)
//	return out
//}
//
//func (ev *evaluator) VectorAnd(lhs, rhs Vector, matching *parser.VectorMatching, lhsh, rhsh []EvalSeriesHelper, enh *EvalNodeHelper) Vector {
//	if matching.Card != parser.CardManyToMany {
//		panic("set operations must only use many-to-many matching")
//	}
//	if len(lhs) == 0 || len(rhs) == 0 {
//		return nil // Short-circuit: AND with nothing is nothing.
//	}
//
//	// The set of signatures for the right-hand side Vector.
//	rightSigs := map[string]struct{}{}
//	// Add all rhs samples to a map so we can easily find matches later.
//	for _, sh := range rhsh {
//		rightSigs[sh.signature] = struct{}{}
//	}
//
//	for i, ls := range lhs {
//		// If there's a matching entry in the right-hand side Vector, add the sample.
//		if _, ok := rightSigs[lhsh[i].signature]; ok {
//			enh.Out = append(enh.Out, ls)
//		}
//	}
//	return enh.Out
//}
//
//func (ev *evaluator) VectorOr(lhs, rhs Vector, matching *parser.VectorMatching, lhsh, rhsh []EvalSeriesHelper, enh *EvalNodeHelper) Vector {
//	if matching.Card != parser.CardManyToMany {
//		panic("set operations must only use many-to-many matching")
//	}
//	if len(lhs) == 0 { // Short-circuit.
//		enh.Out = append(enh.Out, rhs...)
//		return enh.Out
//	} else if len(rhs) == 0 {
//		enh.Out = append(enh.Out, lhs...)
//		return enh.Out
//	}
//
//	leftSigs := map[string]struct{}{}
//	// Add everything from the left-hand-side Vector.
//	for i, ls := range lhs {
//		leftSigs[lhsh[i].signature] = struct{}{}
//		enh.Out = append(enh.Out, ls)
//	}
//	// Add all right-hand side elements which have not been added from the left-hand side.
//	for j, rs := range rhs {
//		if _, ok := leftSigs[rhsh[j].signature]; !ok {
//			enh.Out = append(enh.Out, rs)
//		}
//	}
//	return enh.Out
//}
//
//func (ev *evaluator) VectorUnless(lhs, rhs Vector, matching *parser.VectorMatching, lhsh, rhsh []EvalSeriesHelper, enh *EvalNodeHelper) Vector {
//	if matching.Card != parser.CardManyToMany {
//		panic("set operations must only use many-to-many matching")
//	}
//	// Short-circuit: empty rhs means we will return everything in lhs;
//	// empty lhs means we will return empty - don't need to build a map.
//	if len(lhs) == 0 || len(rhs) == 0 {
//		enh.Out = append(enh.Out, lhs...)
//		return enh.Out
//	}
//
//	rightSigs := map[string]struct{}{}
//	for _, sh := range rhsh {
//		rightSigs[sh.signature] = struct{}{}
//	}
//
//	for i, ls := range lhs {
//		if _, ok := rightSigs[lhsh[i].signature]; !ok {
//			enh.Out = append(enh.Out, ls)
//		}
//	}
//	return enh.Out
//}
//
//// VectorBinop evaluates a binary operation between two Vectors, excluding set operators.
//func (ev *evaluator) VectorBinop(op parser.ItemType, lhs, rhs Vector, matching *parser.VectorMatching, returnBool bool, lhsh, rhsh []EvalSeriesHelper, enh *EvalNodeHelper) Vector {
//	if matching.Card == parser.CardManyToMany {
//		panic("many-to-many only allowed for set operators")
//	}
//	if len(lhs) == 0 || len(rhs) == 0 {
//		return nil // Short-circuit: nothing is going to match.
//	}
//
//	// The control flow below handles one-to-one or many-to-one matching.
//	// For one-to-many, swap sidedness and account for the swap when calculating
//	// values.
//	if matching.Card == parser.CardOneToMany {
//		lhs, rhs = rhs, lhs
//		lhsh, rhsh = rhsh, lhsh
//	}
//
//	// All samples from the rhs hashed by the matching label/values.
//	if enh.rightSigs == nil {
//		enh.rightSigs = make(map[string]Sample, len(enh.Out))
//	} else {
//		for k := range enh.rightSigs {
//			delete(enh.rightSigs, k)
//		}
//	}
//	rightSigs := enh.rightSigs
//
//	// Add all rhs samples to a map so we can easily find matches later.
//	for i, rs := range rhs {
//		sig := rhsh[i].signature
//		// The rhs is guaranteed to be the 'one' side. Having multiple samples
//		// with the same signature means that the matching is many-to-many.
//		if duplSample, found := rightSigs[sig]; found {
//			// oneSide represents which side of the vector represents the 'one' in the many-to-one relationship.
//			oneSide := "right"
//			if matching.Card == parser.CardOneToMany {
//				oneSide = "left"
//			}
//			matchedLabels := rs.Metric.MatchLabels(matching.On, matching.MatchingLabels...)
//			// Many-to-many matching not allowed.
//			ev.errorf("found duplicate series for the match group %s on the %s hand-side of the operation: [%s, %s]"+
//				";many-to-many matching not allowed: matching labels must be unique on one side", matchedLabels.String(), oneSide, rs.Metric.String(), duplSample.Metric.String())
//		}
//		rightSigs[sig] = rs
//	}
//
//	// Tracks the match-signature. For one-to-one operations the value is nil. For many-to-one
//	// the value is a set of signatures to detect duplicated result elements.
//	if enh.matchedSigs == nil {
//		enh.matchedSigs = make(map[string]map[uint64]struct{}, len(rightSigs))
//	} else {
//		for k := range enh.matchedSigs {
//			delete(enh.matchedSigs, k)
//		}
//	}
//	matchedSigs := enh.matchedSigs
//
//	// For all lhs samples find a respective rhs sample and perform
//	// the binary operation.
//	for i, ls := range lhs {
//		sig := lhsh[i].signature
//
//		rs, found := rightSigs[sig] // Look for a match in the rhs Vector.
//		if !found {
//			continue
//		}
//
//		// Account for potentially swapped sidedness.
//		vl, vr := ls.V, rs.V
//		hl, hr := ls.H, rs.H
//		if matching.Card == parser.CardOneToMany {
//			vl, vr = vr, vl
//			hl, hr = hr, hl
//		}
//		value, histogramValue, keep := vectorElemBinop(op, vl, vr, hl, hr)
//		if returnBool {
//			if keep {
//				value = 1.0
//			} else {
//				value = 0.0
//			}
//		} else if !keep {
//			continue
//		}
//		metric := resultMetric(ls.Metric, rs.Metric, op, matching, enh)
//		if returnBool {
//			metric = enh.DropMetricName(metric)
//		}
//		insertedSigs, exists := matchedSigs[sig]
//		if matching.Card == parser.CardOneToOne {
//			if exists {
//				ev.errorf("multiple matches for labels: many-to-one matching must be explicit (group_left/group_right)")
//			}
//			matchedSigs[sig] = nil // Set existence to true.
//		} else {
//			// In many-to-one matching the grouping labels have to ensure a unique metric
//			// for the result Vector. Check whether those labels have already been added for
//			// the same matching labels.
//			insertSig := metric.Hash()
//
//			if !exists {
//				insertedSigs = map[uint64]struct{}{}
//				matchedSigs[sig] = insertedSigs
//			} else if _, duplicate := insertedSigs[insertSig]; duplicate {
//				ev.errorf("multiple matches for labels: grouping labels must ensure unique matches")
//			}
//			insertedSigs[insertSig] = struct{}{}
//		}
//
//		if (hl != nil && hr != nil) || (hl == nil && hr == nil) {
//			// Both lhs and rhs are of same type.
//			enh.Out = append(enh.Out, Sample{
//				Metric: metric,
//				Point:  Point{V: value, H: histogramValue},
//			})
//		}
//	}
//	return enh.Out
//}
//
//func signatureFunc(on bool, b []byte, names ...string) func(labels.Labels) string {
//	if on {
//		slices.Sort(names)
//		return func(lset labels.Labels) string {
//			return string(lset.BytesWithLabels(b, names...))
//		}
//	}
//	names = append([]string{labels.MetricName}, names...)
//	slices.Sort(names)
//	return func(lset labels.Labels) string {
//		return string(lset.BytesWithoutLabels(b, names...))
//	}
//}
//
//// resultMetric returns the metric for the given sample(s) based on the Vector
//// binary operation and the matching options.
//func resultMetric(lhs, rhs labels.Labels, op parser.ItemType, matching *parser.VectorMatching, enh *EvalNodeHelper) labels.Labels {
//	if enh.resultMetric == nil {
//		enh.resultMetric = make(map[string]labels.Labels, len(enh.Out))
//	}
//
//	enh.resetBuilder(lhs)
//	buf := bytes.NewBuffer(enh.lblResultBuf[:0])
//	enh.lblBuf = lhs.Bytes(enh.lblBuf)
//	buf.Write(enh.lblBuf)
//	enh.lblBuf = rhs.Bytes(enh.lblBuf)
//	buf.Write(enh.lblBuf)
//	enh.lblResultBuf = buf.Bytes()
//
//	if ret, ok := enh.resultMetric[string(enh.lblResultBuf)]; ok {
//		return ret
//	}
//	str := string(enh.lblResultBuf)
//
//	if shouldDropMetricName(op) {
//		enh.lb.Del(labels.MetricName)
//	}
//
//	if matching.Card == parser.CardOneToOne {
//		if matching.On {
//			enh.lb.Keep(matching.MatchingLabels...)
//		} else {
//			enh.lb.Del(matching.MatchingLabels...)
//		}
//	}
//	for _, ln := range matching.Include {
//		// Included labels from the `group_x` modifier are taken from the "one"-side.
//		if v := rhs.Get(ln); v != "" {
//			enh.lb.Set(ln, v)
//		} else {
//			enh.lb.Del(ln)
//		}
//	}
//
//	ret := enh.lb.Labels(labels.EmptyLabels())
//	enh.resultMetric[str] = ret
//	return ret
//}
//
//// VectorscalarBinop evaluates a binary operation between a Vector and a Scalar.
//func (ev *evaluator) VectorscalarBinop(op parser.ItemType, lhs Vector, rhs Scalar, swap, returnBool bool, enh *EvalNodeHelper) Vector {
//	for _, lhsSample := range lhs {
//		lv, rv := lhsSample.V, rhs.V
//		// lhs always contains the Vector. If the original position was different
//		// swap for calculating the value.
//		if swap {
//			lv, rv = rv, lv
//		}
//		value, _, keep := vectorElemBinop(op, lv, rv, nil, nil)
//		// Catch cases where the scalar is the LHS in a scalar-vector comparison operation.
//		// We want to always keep the vector element value as the output value, even if it's on the RHS.
//		if op.IsComparisonOperator() && swap {
//			value = rv
//		}
//		if returnBool {
//			if keep {
//				value = 1.0
//			} else {
//				value = 0.0
//			}
//			keep = true
//		}
//		if keep {
//			lhsSample.V = value
//			if shouldDropMetricName(op) || returnBool {
//				lhsSample.Metric = enh.DropMetricName(lhsSample.Metric)
//			}
//			enh.Out = append(enh.Out, lhsSample)
//		}
//	}
//	return enh.Out
//}
//
//func dropMetricName(l labels.Labels) labels.Labels {
//	return labels.NewBuilder(l).Del(labels.MetricName).Labels(labels.EmptyLabels())
//}
//
//// scalarBinop evaluates a binary operation between two Scalars.
//func scalarBinop(op parser.ItemType, lhs, rhs float64) float64 {
//	switch op {
//	case parser.ADD:
//		return lhs + rhs
//	case parser.SUB:
//		return lhs - rhs
//	case parser.MUL:
//		return lhs * rhs
//	case parser.DIV:
//		return lhs / rhs
//	case parser.POW:
//		return math.Pow(lhs, rhs)
//	case parser.MOD:
//		return math.Mod(lhs, rhs)
//	case parser.EQLC:
//		return btos(lhs == rhs)
//	case parser.NEQ:
//		return btos(lhs != rhs)
//	case parser.GTR:
//		return btos(lhs > rhs)
//	case parser.LSS:
//		return btos(lhs < rhs)
//	case parser.GTE:
//		return btos(lhs >= rhs)
//	case parser.LTE:
//		return btos(lhs <= rhs)
//	case parser.ATAN2:
//		return math.Atan2(lhs, rhs)
//	}
//	panic(fmt.Errorf("operator %q not allowed for Scalar operations", op))
//}
//
//// vectorElemBinop evaluates a binary operation between two Vector elements.
//func vectorElemBinop(op parser.ItemType, lhs, rhs float64, hlhs, hrhs *histogram.FloatHistogram) (float64, *histogram.FloatHistogram, bool) {
//	switch op {
//	case parser.ADD:
//		if hlhs != nil && hrhs != nil {
//			// The histogram being added must have the larger schema
//			// code (i.e. the higher resolution).
//			if hrhs.Schema >= hlhs.Schema {
//				return 0, hlhs.Copy().Add(hrhs), true
//			}
//			return 0, hrhs.Copy().Add(hlhs), true
//		}
//		return lhs + rhs, nil, true
//	case parser.SUB:
//		return lhs - rhs, nil, true
//	case parser.MUL:
//		return lhs * rhs, nil, true
//	case parser.DIV:
//		return lhs / rhs, nil, true
//	case parser.POW:
//		return math.Pow(lhs, rhs), nil, true
//	case parser.MOD:
//		return math.Mod(lhs, rhs), nil, true
//	case parser.EQLC:
//		return lhs, nil, lhs == rhs
//	case parser.NEQ:
//		return lhs, nil, lhs != rhs
//	case parser.GTR:
//		return lhs, nil, lhs > rhs
//	case parser.LSS:
//		return lhs, nil, lhs < rhs
//	case parser.GTE:
//		return lhs, nil, lhs >= rhs
//	case parser.LTE:
//		return lhs, nil, lhs <= rhs
//	case parser.ATAN2:
//		return math.Atan2(lhs, rhs), nil, true
//	}
//	panic(fmt.Errorf("operator %q not allowed for operations between Vectors", op))
//}
//
//type groupedAggregation struct {
//	hasFloat       bool // Has at least 1 float64 sample aggregated.
//	hasHistogram   bool // Has at least 1 histogram sample aggregated.
//	labels         labels.Labels
//	value          float64
//	histogramValue *histogram.FloatHistogram
//	mean           float64
//	groupCount     int
//	heap           vectorByValueHeap
//	reverseHeap    vectorByReverseValueHeap
//}
//
//// aggregation evaluates an aggregation operation on a Vector. The provided grouping labels
//// must be sorted.
//func (ev *evaluator) aggregation(op parser.ItemType, grouping []string, without bool, param interface{}, vec Vector, seriesHelper []EvalSeriesHelper, enh *EvalNodeHelper) Vector {
//	result := map[uint64]*groupedAggregation{}
//	orderedResult := []*groupedAggregation{}
//	var k int64
//	if op == parser.TOPK || op == parser.BOTTOMK {
//		f := param.(float64)
//		if !convertibleToInt64(f) {
//			ev.errorf("Scalar value %v overflows int64", f)
//		}
//		k = int64(f)
//		if k < 1 {
//			return Vector{}
//		}
//	}
//	var q float64
//	if op == parser.QUANTILE {
//		q = param.(float64)
//	}
//	var valueLabel string
//	var recomputeGroupingKey bool
//	if op == parser.COUNT_VALUES {
//		valueLabel = param.(string)
//		if !model.LabelName(valueLabel).IsValid() {
//			ev.errorf("invalid label name %q", valueLabel)
//		}
//		if !without {
//			// We're changing the grouping labels so we have to ensure they're still sorted
//			// and we have to flag to recompute the grouping key. Considering the count_values()
//			// operator is less frequently used than other aggregations, we're fine having to
//			// re-compute the grouping key on each step for this case.
//			grouping = append(grouping, valueLabel)
//			slices.Sort(grouping)
//			recomputeGroupingKey = true
//		}
//	}
//
//	var buf []byte
//	for si, s := range vec {
//		metric := s.Metric
//
//		if op == parser.COUNT_VALUES {
//			enh.resetBuilder(metric)
//			enh.lb.Set(valueLabel, strconv.FormatFloat(s.V, 'f', -1, 64))
//			metric = enh.lb.Labels(labels.EmptyLabels())
//
//			// We've changed the metric so we have to recompute the grouping key.
//			recomputeGroupingKey = true
//		}
//
//		// We can use the pre-computed grouping key unless grouping labels have changed.
//		var groupingKey uint64
//		if !recomputeGroupingKey {
//			groupingKey = seriesHelper[si].groupingKey
//		} else {
//			groupingKey, buf = generateGroupingKey(metric, grouping, without, buf)
//		}
//
//		group, ok := result[groupingKey]
//		// Add a new group if it doesn't exist.
//		if !ok {
//			var m labels.Labels
//			enh.resetBuilder(metric)
//			if without {
//				enh.lb.Del(grouping...)
//				enh.lb.Del(labels.MetricName)
//				m = enh.lb.Labels(labels.EmptyLabels())
//			} else if len(grouping) > 0 {
//				enh.lb.Keep(grouping...)
//				m = enh.lb.Labels(labels.EmptyLabels())
//			} else {
//				m = labels.EmptyLabels()
//			}
//			newAgg := &groupedAggregation{
//				labels:     m,
//				value:      s.V,
//				mean:       s.V,
//				groupCount: 1,
//			}
//			if s.H == nil {
//				newAgg.hasFloat = true
//			} else if op == parser.SUM {
//				newAgg.histogramValue = s.H.Copy()
//				newAgg.hasHistogram = true
//			}
//
//			result[groupingKey] = newAgg
//			orderedResult = append(orderedResult, newAgg)
//
//			inputVecLen := int64(len(vec))
//			resultSize := k
//			if k > inputVecLen {
//				resultSize = inputVecLen
//			} else if k == 0 {
//				resultSize = 1
//			}
//			switch op {
//			case parser.STDVAR, parser.STDDEV:
//				result[groupingKey].value = 0
//			case parser.TOPK, parser.QUANTILE:
//				result[groupingKey].heap = make(vectorByValueHeap, 1, resultSize)
//				result[groupingKey].heap[0] = Sample{
//					Point:  Point{V: s.V},
//					Metric: s.Metric,
//				}
//			case parser.BOTTOMK:
//				result[groupingKey].reverseHeap = make(vectorByReverseValueHeap, 1, resultSize)
//				result[groupingKey].reverseHeap[0] = Sample{
//					Point:  Point{V: s.V},
//					Metric: s.Metric,
//				}
//			case parser.GROUP:
//				result[groupingKey].value = 1
//			}
//			continue
//		}
//
//		switch op {
//		case parser.SUM:
//			if s.H != nil {
//				group.hasHistogram = true
//				if group.histogramValue != nil {
//					// The histogram being added must have
//					// an equal or larger schema.
//					if s.H.Schema >= group.histogramValue.Schema {
//						group.histogramValue.Add(s.H)
//					} else {
//						h := s.H.Copy()
//						h.Add(group.histogramValue)
//						group.histogramValue = h
//					}
//				}
//				// Otherwise the aggregation contained floats
//				// previously and will be invalid anyway. No
//				// point in copying the histogram in that case.
//			} else {
//				group.hasFloat = true
//				group.value += s.V
//			}
//
//		case parser.AVG:
//			group.groupCount++
//			if math.IsInf(group.mean, 0) {
//				if math.IsInf(s.V, 0) && (group.mean > 0) == (s.V > 0) {
//					// The `mean` and `s.V` values are `Inf` of the same sign.  They
//					// can't be subtracted, but the value of `mean` is correct
//					// already.
//					break
//				}
//				if !math.IsInf(s.V, 0) && !math.IsNaN(s.V) {
//					// At this stage, the mean is an infinite. If the added
//					// value is neither an Inf or a Nan, we can keep that mean
//					// value.
//					// This is required because our calculation below removes
//					// the mean value, which would look like Inf += x - Inf and
//					// end up as a NaN.
//					break
//				}
//			}
//			// Divide each side of the `-` by `group.groupCount` to avoid float64 overflows.
//			group.mean += s.V/float64(group.groupCount) - group.mean/float64(group.groupCount)
//
//		case parser.GROUP:
//			// Do nothing. Required to avoid the panic in `default:` below.
//
//		case parser.MAX:
//			if group.value < s.V || math.IsNaN(group.value) {
//				group.value = s.V
//			}
//
//		case parser.MIN:
//			if group.value > s.V || math.IsNaN(group.value) {
//				group.value = s.V
//			}
//
//		case parser.COUNT, parser.COUNT_VALUES:
//			group.groupCount++
//
//		case parser.STDVAR, parser.STDDEV:
//			group.groupCount++
//			delta := s.V - group.mean
//			group.mean += delta / float64(group.groupCount)
//			group.value += delta * (s.V - group.mean)
//
//		case parser.TOPK:
//			if int64(len(group.heap)) < k || group.heap[0].V < s.V || math.IsNaN(group.heap[0].V) {
//				if int64(len(group.heap)) == k {
//					if k == 1 { // For k==1 we can replace in-situ.
//						group.heap[0] = Sample{
//							Point:  Point{V: s.V},
//							Metric: s.Metric,
//						}
//						break
//					}
//					heap.Pop(&group.heap)
//				}
//				heap.Push(&group.heap, &Sample{
//					Point:  Point{V: s.V},
//					Metric: s.Metric,
//				})
//			}
//
//		case parser.BOTTOMK:
//			if int64(len(group.reverseHeap)) < k || group.reverseHeap[0].V > s.V || math.IsNaN(group.reverseHeap[0].V) {
//				if int64(len(group.reverseHeap)) == k {
//					if k == 1 { // For k==1 we can replace in-situ.
//						group.reverseHeap[0] = Sample{
//							Point:  Point{V: s.V},
//							Metric: s.Metric,
//						}
//						break
//					}
//					heap.Pop(&group.reverseHeap)
//				}
//				heap.Push(&group.reverseHeap, &Sample{
//					Point:  Point{V: s.V},
//					Metric: s.Metric,
//				})
//			}
//
//		case parser.QUANTILE:
//			group.heap = append(group.heap, s)
//
//		default:
//			panic(fmt.Errorf("expected aggregation operator but got %q", op))
//		}
//	}
//
//	// Construct the result Vector from the aggregated groups.
//	for _, aggr := range orderedResult {
//		switch op {
//		case parser.AVG:
//			aggr.value = aggr.mean
//
//		case parser.COUNT, parser.COUNT_VALUES:
//			aggr.value = float64(aggr.groupCount)
//
//		case parser.STDVAR:
//			aggr.value = aggr.value / float64(aggr.groupCount)
//
//		case parser.STDDEV:
//			aggr.value = math.Sqrt(aggr.value / float64(aggr.groupCount))
//
//		case parser.TOPK:
//			// The heap keeps the lowest value on top, so reverse it.
//			if len(aggr.heap) > 1 {
//				sort.Sort(sort.Reverse(aggr.heap))
//			}
//			for _, v := range aggr.heap {
//				enh.Out = append(enh.Out, Sample{
//					Metric: v.Metric,
//					Point:  Point{V: v.V},
//				})
//			}
//			continue // Bypass default append.
//
//		case parser.BOTTOMK:
//			// The heap keeps the highest value on top, so reverse it.
//			if len(aggr.reverseHeap) > 1 {
//				sort.Sort(sort.Reverse(aggr.reverseHeap))
//			}
//			for _, v := range aggr.reverseHeap {
//				enh.Out = append(enh.Out, Sample{
//					Metric: v.Metric,
//					Point:  Point{V: v.V},
//				})
//			}
//			continue // Bypass default append.
//
//		case parser.QUANTILE:
//			aggr.value = quantile(q, aggr.heap)
//
//		case parser.SUM:
//			if aggr.hasFloat && aggr.hasHistogram {
//				// We cannot aggregate histogram sample with a float64 sample.
//				continue
//			}
//		default:
//			// For other aggregations, we already have the right value.
//		}
//
//		enh.Out = append(enh.Out, Sample{
//			Metric: aggr.labels,
//			Point:  Point{V: aggr.value, H: aggr.histogramValue},
//		})
//	}
//	return enh.Out
//}
//
//// groupingKey builds and returns the grouping key for the given metric and
//// grouping labels.
//func generateGroupingKey(metric labels.Labels, grouping []string, without bool, buf []byte) (uint64, []byte) {
//	if without {
//		return metric.HashWithoutLabels(buf, grouping...)
//	}
//
//	if len(grouping) == 0 {
//		// No need to generate any hash if there are no grouping labels.
//		return 0, buf
//	}
//
//	return metric.HashForLabels(buf, grouping...)
//}
//
//// btos returns 1 if b is true, 0 otherwise.
//func btos(b bool) float64 {
//	if b {
//		return 1
//	}
//	return 0
//}
//
//// shouldDropMetricName returns whether the metric name should be dropped in the
//// result of the op operation.
//func shouldDropMetricName(op parser.ItemType) bool {
//	switch op {
//	case parser.ADD, parser.SUB, parser.DIV, parser.MUL, parser.POW, parser.MOD:
//		return true
//	default:
//		return false
//	}
//}
//
//// unwrapParenExpr does the AST equivalent of removing parentheses around a expression.
//func unwrapParenExpr(e *parser.Expr) {
//	for {
//		if p, ok := (*e).(*parser.ParenExpr); ok {
//			*e = p.Expr
//		} else {
//			break
//		}
//	}
//}
//
//func unwrapStepInvariantExpr(e parser.Expr) parser.Expr {
//	if p, ok := e.(*parser.StepInvariantExpr); ok {
//		return p.Expr
//	}
//	return e
//}
