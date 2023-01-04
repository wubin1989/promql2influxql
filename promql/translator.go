package promql

import (
	"fmt"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/promql/parser"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb/chunkenc"
	"github.com/prometheus/prometheus/util/stats"
	"github.com/wubin1989/promql2influxql/command"
	"go.opentelemetry.io/otel"
	"golang.org/x/exp/slices"
	"reflect"
	"time"
)

var _ command.ITranslator = (*translator)(nil)

type translator struct {
}

func (t translator) Translate() (influxql string, ok bool) {
	// This is the top-level evaluation method.
	// Thus, we check for timeout/cancellation here.
	if err := contextDone(ev.ctx, "expression evaluation"); err != nil {
		ev.error(err)
	}
	numSteps := int((ev.endTimestamp-ev.startTimestamp)/ev.interval) + 1

	// Create a new span to help investigate inner evaluation performances.
	ctxWithSpan, span := otel.Tracer("").Start(ev.ctx, stats.InnerEvalTime.SpanOperation()+" eval "+reflect.TypeOf(expr).String())
	ev.ctx = ctxWithSpan
	defer span.End()

	switch e := expr.(type) {
	case *parser.AggregateExpr:
		// Grouping labels must be sorted (expected both by generateGroupingKey() and aggregation()).
		sortedGrouping := e.Grouping
		slices.Sort(sortedGrouping)

		// Prepare a function to initialise series helpers with the grouping key.
		buf := make([]byte, 0, 1024)
		initSeries := func(series labels.Labels, h *EvalSeriesHelper) {
			h.groupingKey, buf = generateGroupingKey(series, sortedGrouping, e.Without, buf)
		}

		unwrapParenExpr(&e.Param)
		param := unwrapStepInvariantExpr(e.Param)
		unwrapParenExpr(&param)
		if s, ok := param.(*parser.StringLiteral); ok {
			return ev.rangeEval(initSeries, func(v []parser.Value, sh [][]EvalSeriesHelper, enh *EvalNodeHelper) (Vector, storage.Warnings) {
				return ev.aggregation(e.Op, sortedGrouping, e.Without, s.Val, v[0].(Vector), sh[0], enh), nil
			}, e.Expr)
		}

		return ev.rangeEval(initSeries, func(v []parser.Value, sh [][]EvalSeriesHelper, enh *EvalNodeHelper) (Vector, storage.Warnings) {
			var param float64
			if e.Param != nil {
				param = v[0].(Vector)[0].V
			}
			return ev.aggregation(e.Op, sortedGrouping, e.Without, param, v[1].(Vector), sh[1], enh), nil
		}, e.Param, e.Expr)

	case *parser.Call:
		call := FunctionCalls[e.Func.Name]
		if e.Func.Name == "timestamp" {
			// Matrix evaluation always returns the evaluation time,
			// so this function needs special handling when given
			// a vector selector.
			unwrapParenExpr(&e.Args[0])
			arg := unwrapStepInvariantExpr(e.Args[0])
			unwrapParenExpr(&arg)
			vs, ok := arg.(*parser.VectorSelector)
			if ok {
				return ev.rangeEval(nil, func(v []parser.Value, _ [][]EvalSeriesHelper, enh *EvalNodeHelper) (Vector, storage.Warnings) {
					if vs.Timestamp != nil {
						// This is a special case only for "timestamp" since the offset
						// needs to be adjusted for every point.
						vs.Offset = time.Duration(enh.Ts-*vs.Timestamp) * time.Millisecond
					}
					val, ws := ev.vectorSelector(vs, enh.Ts)
					return call([]parser.Value{val}, e.Args, enh), ws
				})
			}
		}

		// Check if the function has a matrix argument.
		var (
			matrixArgIndex int
			matrixArg      bool
			warnings       storage.Warnings
		)
		for i := range e.Args {
			unwrapParenExpr(&e.Args[i])
			a := unwrapStepInvariantExpr(e.Args[i])
			unwrapParenExpr(&a)
			if _, ok := a.(*parser.MatrixSelector); ok {
				matrixArgIndex = i
				matrixArg = true
				break
			}
			// parser.SubqueryExpr can be used in place of parser.MatrixSelector.
			if subq, ok := a.(*parser.SubqueryExpr); ok {
				matrixArgIndex = i
				matrixArg = true
				// Replacing parser.SubqueryExpr with parser.MatrixSelector.
				val, totalSamples, ws := ev.evalSubquery(subq)
				e.Args[i] = val
				warnings = append(warnings, ws...)
				defer func() {
					// subquery result takes space in the memory. Get rid of that at the end.
					val.VectorSelector.(*parser.VectorSelector).Series = nil
					ev.currentSamples -= totalSamples
				}()
				break
			}
		}
		if !matrixArg {
			// Does not have a matrix argument.
			return ev.rangeEval(nil, func(v []parser.Value, _ [][]EvalSeriesHelper, enh *EvalNodeHelper) (Vector, storage.Warnings) {
				return call(v, e.Args, enh), warnings
			}, e.Args...)
		}

		inArgs := make([]parser.Value, len(e.Args))
		// Evaluate any non-matrix arguments.
		otherArgs := make([]Matrix, len(e.Args))
		otherInArgs := make([]Vector, len(e.Args))
		for i, e := range e.Args {
			if i != matrixArgIndex {
				val, ws := ev.eval(e)
				otherArgs[i] = val.(Matrix)
				otherInArgs[i] = Vector{Sample{}}
				inArgs[i] = otherInArgs[i]
				warnings = append(warnings, ws...)
			}
		}

		unwrapParenExpr(&e.Args[matrixArgIndex])
		arg := unwrapStepInvariantExpr(e.Args[matrixArgIndex])
		unwrapParenExpr(&arg)
		sel := arg.(*parser.MatrixSelector)
		selVS := sel.VectorSelector.(*parser.VectorSelector)

		ws, err := checkAndExpandSeriesSet(ev.ctx, sel)
		warnings = append(warnings, ws...)
		if err != nil {
			ev.error(errWithWarnings{fmt.Errorf("expanding series: %w", err), warnings})
		}
		mat := make(Matrix, 0, len(selVS.Series)) // Output matrix.
		offset := durationMilliseconds(selVS.Offset)
		selRange := durationMilliseconds(sel.Range)
		stepRange := selRange
		if stepRange > ev.interval {
			stepRange = ev.interval
		}
		// Reuse objects across steps to save memory allocations.
		points := getPointSlice(16)
		inMatrix := make(Matrix, 1)
		inArgs[matrixArgIndex] = inMatrix
		enh := &EvalNodeHelper{Out: make(Vector, 0, 1)}
		// Process all the calls for one time series at a time.
		it := storage.NewBuffer(selRange)
		var chkIter chunkenc.Iterator
		for i, s := range selVS.Series {
			ev.currentSamples -= len(points)
			points = points[:0]
			chkIter = s.Iterator(chkIter)
			it.Reset(chkIter)
			metric := selVS.Series[i].Labels()
			// The last_over_time function acts like offset; thus, it
			// should keep the metric name.  For all the other range
			// vector functions, the only change needed is to drop the
			// metric name in the output.
			if e.Func.Name != "last_over_time" {
				metric = dropMetricName(metric)
			}
			ss := Series{
				Metric: metric,
				Points: getPointSlice(numSteps),
			}
			inMatrix[0].Metric = selVS.Series[i].Labels()
			for ts, step := ev.startTimestamp, -1; ts <= ev.endTimestamp; ts += ev.interval {
				step++
				// Set the non-matrix arguments.
				// They are scalar, so it is safe to use the step number
				// when looking up the argument, as there will be no gaps.
				for j := range e.Args {
					if j != matrixArgIndex {
						otherInArgs[j][0].V = otherArgs[j][0].Points[step].V
					}
				}
				maxt := ts - offset
				mint := maxt - selRange
				// Evaluate the matrix selector for this series for this step.
				points = ev.matrixIterSlice(it, mint, maxt, points)
				if len(points) == 0 {
					continue
				}
				inMatrix[0].Points = points
				enh.Ts = ts
				// Make the function call.
				outVec := call(inArgs, e.Args, enh)
				ev.samplesStats.IncrementSamplesAtStep(step, int64(len(points)))
				enh.Out = outVec[:0]
				if len(outVec) > 0 {
					ss.Points = append(ss.Points, Point{V: outVec[0].Point.V, H: outVec[0].Point.H, T: ts})
				}
				// Only buffer stepRange milliseconds from the second step on.
				it.ReduceDelta(stepRange)
			}
			if len(ss.Points) > 0 {
				if ev.currentSamples+len(ss.Points) <= ev.maxSamples {
					mat = append(mat, ss)
					ev.currentSamples += len(ss.Points)
				} else {
					ev.error(ErrTooManySamples(env))
				}
			} else {
				putPointSlice(ss.Points)
			}
			ev.samplesStats.UpdatePeak(ev.currentSamples)
		}
		ev.samplesStats.UpdatePeak(ev.currentSamples)

		ev.currentSamples -= len(points)
		putPointSlice(points)

		// The absent_over_time function returns 0 or 1 series. So far, the matrix
		// contains multiple series. The following code will create a new series
		// with values of 1 for the timestamps where no series has value.
		if e.Func.Name == "absent_over_time" {
			steps := int(1 + (ev.endTimestamp-ev.startTimestamp)/ev.interval)
			// Iterate once to look for a complete series.
			for _, s := range mat {
				if len(s.Points) == steps {
					return Matrix{}, warnings
				}
			}

			found := map[int64]struct{}{}

			for i, s := range mat {
				for _, p := range s.Points {
					found[p.T] = struct{}{}
				}
				if i > 0 && len(found) == steps {
					return Matrix{}, warnings
				}
			}

			newp := make([]Point, 0, steps-len(found))
			for ts := ev.startTimestamp; ts <= ev.endTimestamp; ts += ev.interval {
				if _, ok := found[ts]; !ok {
					newp = append(newp, Point{T: ts, V: 1})
				}
			}

			return Matrix{
				Series{
					Metric: createLabelsForAbsentFunction(e.Args[0]),
					Points: newp,
				},
			}, warnings
		}

		if mat.ContainsSameLabelset() {
			ev.errorf("vector cannot contain metrics with the same labelset")
		}

		return mat, warnings

	case *parser.ParenExpr:
		return ev.eval(e.Expr)

	case *parser.UnaryExpr:
		val, ws := ev.eval(e.Expr)
		mat := val.(Matrix)
		if e.Op == parser.SUB {
			for i := range mat {
				mat[i].Metric = dropMetricName(mat[i].Metric)
				for j := range mat[i].Points {
					mat[i].Points[j].V = -mat[i].Points[j].V
				}
			}
			if mat.ContainsSameLabelset() {
				ev.errorf("vector cannot contain metrics with the same labelset")
			}
		}
		return mat, ws

	case *parser.BinaryExpr:
		switch lt, rt := e.LHS.Type(), e.RHS.Type(); {
		case lt == parser.ValueTypeScalar && rt == parser.ValueTypeScalar:
			return ev.rangeEval(nil, func(v []parser.Value, _ [][]EvalSeriesHelper, enh *EvalNodeHelper) (Vector, storage.Warnings) {
				val := scalarBinop(e.Op, v[0].(Vector)[0].Point.V, v[1].(Vector)[0].Point.V)
				return append(enh.Out, Sample{Point: Point{V: val}}), nil
			}, e.LHS, e.RHS)
		case lt == parser.ValueTypeVector && rt == parser.ValueTypeVector:
			// Function to compute the join signature for each series.
			buf := make([]byte, 0, 1024)
			sigf := signatureFunc(e.VectorMatching.On, buf, e.VectorMatching.MatchingLabels...)
			initSignatures := func(series labels.Labels, h *EvalSeriesHelper) {
				h.signature = sigf(series)
			}
			switch e.Op {
			case parser.LAND:
				return ev.rangeEval(initSignatures, func(v []parser.Value, sh [][]EvalSeriesHelper, enh *EvalNodeHelper) (Vector, storage.Warnings) {
					return ev.VectorAnd(v[0].(Vector), v[1].(Vector), e.VectorMatching, sh[0], sh[1], enh), nil
				}, e.LHS, e.RHS)
			case parser.LOR:
				return ev.rangeEval(initSignatures, func(v []parser.Value, sh [][]EvalSeriesHelper, enh *EvalNodeHelper) (Vector, storage.Warnings) {
					return ev.VectorOr(v[0].(Vector), v[1].(Vector), e.VectorMatching, sh[0], sh[1], enh), nil
				}, e.LHS, e.RHS)
			case parser.LUNLESS:
				return ev.rangeEval(initSignatures, func(v []parser.Value, sh [][]EvalSeriesHelper, enh *EvalNodeHelper) (Vector, storage.Warnings) {
					return ev.VectorUnless(v[0].(Vector), v[1].(Vector), e.VectorMatching, sh[0], sh[1], enh), nil
				}, e.LHS, e.RHS)
			default:
				return ev.rangeEval(initSignatures, func(v []parser.Value, sh [][]EvalSeriesHelper, enh *EvalNodeHelper) (Vector, storage.Warnings) {
					return ev.VectorBinop(e.Op, v[0].(Vector), v[1].(Vector), e.VectorMatching, e.ReturnBool, sh[0], sh[1], enh), nil
				}, e.LHS, e.RHS)
			}

		case lt == parser.ValueTypeVector && rt == parser.ValueTypeScalar:
			return ev.rangeEval(nil, func(v []parser.Value, _ [][]EvalSeriesHelper, enh *EvalNodeHelper) (Vector, storage.Warnings) {
				return ev.VectorscalarBinop(e.Op, v[0].(Vector), Scalar{V: v[1].(Vector)[0].Point.V}, false, e.ReturnBool, enh), nil
			}, e.LHS, e.RHS)

		case lt == parser.ValueTypeScalar && rt == parser.ValueTypeVector:
			return ev.rangeEval(nil, func(v []parser.Value, _ [][]EvalSeriesHelper, enh *EvalNodeHelper) (Vector, storage.Warnings) {
				return ev.VectorscalarBinop(e.Op, v[1].(Vector), Scalar{V: v[0].(Vector)[0].Point.V}, true, e.ReturnBool, enh), nil
			}, e.LHS, e.RHS)
		}

	case *parser.NumberLiteral:
		return ev.rangeEval(nil, func(v []parser.Value, _ [][]EvalSeriesHelper, enh *EvalNodeHelper) (Vector, storage.Warnings) {
			return append(enh.Out, Sample{Point: Point{V: e.Val}, Metric: labels.EmptyLabels()}), nil
		})

	case *parser.StringLiteral:
		return String{V: e.Val, T: ev.startTimestamp}, nil

	case *parser.VectorSelector:
		ws, err := checkAndExpandSeriesSet(ev.ctx, e)
		if err != nil {
			ev.error(errWithWarnings{fmt.Errorf("expanding series: %w", err), ws})
		}
		mat := make(Matrix, 0, len(e.Series))
		it := storage.NewMemoizedEmptyIterator(durationMilliseconds(ev.lookbackDelta))
		var chkIter chunkenc.Iterator
		for i, s := range e.Series {
			chkIter = s.Iterator(chkIter)
			it.Reset(chkIter)
			ss := Series{
				Metric: e.Series[i].Labels(),
				Points: getPointSlice(numSteps),
			}

			for ts, step := ev.startTimestamp, -1; ts <= ev.endTimestamp; ts += ev.interval {
				step++
				_, v, h, ok := ev.vectorSelectorSingle(it, e, ts)
				if ok {
					if ev.currentSamples < ev.maxSamples {
						ss.Points = append(ss.Points, Point{V: v, H: h, T: ts})
						ev.samplesStats.IncrementSamplesAtStep(step, 1)
						ev.currentSamples++
					} else {
						ev.error(ErrTooManySamples(env))
					}
				}
			}

			if len(ss.Points) > 0 {
				mat = append(mat, ss)
			} else {
				putPointSlice(ss.Points)
			}
		}
		ev.samplesStats.UpdatePeak(ev.currentSamples)
		return mat, ws

	case *parser.MatrixSelector:
		if ev.startTimestamp != ev.endTimestamp {
			panic(errors.New("cannot do range evaluation of matrix selector"))
		}
		return ev.matrixSelector(e)

	case *parser.SubqueryExpr:
		offsetMillis := durationMilliseconds(e.Offset)
		rangeMillis := durationMilliseconds(e.Range)
		newEv := &evaluator{
			endTimestamp:             ev.endTimestamp - offsetMillis,
			ctx:                      ev.ctx,
			currentSamples:           ev.currentSamples,
			maxSamples:               ev.maxSamples,
			logger:                   ev.logger,
			lookbackDelta:            ev.lookbackDelta,
			samplesStats:             ev.samplesStats.NewChild(),
			noStepSubqueryIntervalFn: ev.noStepSubqueryIntervalFn,
		}

		if e.Step != 0 {
			newEv.interval = durationMilliseconds(e.Step)
		} else {
			newEv.interval = ev.noStepSubqueryIntervalFn(rangeMillis)
		}

		// Start with the first timestamp after (ev.startTimestamp - offset - range)
		// that is aligned with the step (multiple of 'newEv.interval').
		newEv.startTimestamp = newEv.interval * ((ev.startTimestamp - offsetMillis - rangeMillis) / newEv.interval)
		if newEv.startTimestamp < (ev.startTimestamp - offsetMillis - rangeMillis) {
			newEv.startTimestamp += newEv.interval
		}

		if newEv.startTimestamp != ev.startTimestamp {
			// Adjust the offset of selectors based on the new
			// start time of the evaluator since the calculation
			// of the offset with @ happens w.r.t. the start time.
			setOffsetForAtModifier(newEv.startTimestamp, e.Expr)
		}

		res, ws := newEv.eval(e.Expr)
		ev.currentSamples = newEv.currentSamples
		ev.samplesStats.UpdatePeakFromSubquery(newEv.samplesStats)
		ev.samplesStats.IncrementSamplesAtTimestamp(ev.endTimestamp, newEv.samplesStats.TotalSamples)
		return res, ws
	case *parser.StepInvariantExpr:
		switch ce := e.Expr.(type) {
		case *parser.StringLiteral, *parser.NumberLiteral:
			return ev.eval(ce)
		}

		newEv := &evaluator{
			startTimestamp:           ev.startTimestamp,
			endTimestamp:             ev.startTimestamp, // Always a single evaluation.
			interval:                 ev.interval,
			ctx:                      ev.ctx,
			currentSamples:           ev.currentSamples,
			maxSamples:               ev.maxSamples,
			logger:                   ev.logger,
			lookbackDelta:            ev.lookbackDelta,
			samplesStats:             ev.samplesStats.NewChild(),
			noStepSubqueryIntervalFn: ev.noStepSubqueryIntervalFn,
		}
		res, ws := newEv.eval(e.Expr)
		ev.currentSamples = newEv.currentSamples
		ev.samplesStats.UpdatePeakFromSubquery(newEv.samplesStats)
		for ts, step := ev.startTimestamp, -1; ts <= ev.endTimestamp; ts = ts + ev.interval {
			step++
			ev.samplesStats.IncrementSamplesAtStep(step, newEv.samplesStats.TotalSamples)
		}
		switch e.Expr.(type) {
		case *parser.MatrixSelector, *parser.SubqueryExpr:
			// We do not duplicate results for range selectors since result is a matrix
			// with their unique timestamps which does not depend on the step.
			return res, ws
		}

		// For every evaluation while the value remains same, the timestamp for that
		// value would change for different eval times. Hence we duplicate the result
		// with changed timestamps.
		mat, ok := res.(Matrix)
		if !ok {
			panic(fmt.Errorf("unexpected result in StepInvariantExpr evaluation: %T", expr))
		}
		for i := range mat {
			if len(mat[i].Points) != 1 {
				panic(fmt.Errorf("unexpected number of samples"))
			}
			for ts := ev.startTimestamp + ev.interval; ts <= ev.endTimestamp; ts = ts + ev.interval {
				mat[i].Points = append(mat[i].Points, Point{
					T: ts,
					V: mat[i].Points[0].V,
					H: mat[i].Points[0].H,
				})
				ev.currentSamples++
				if ev.currentSamples > ev.maxSamples {
					ev.error(ErrTooManySamples(env))
				}
			}
		}
		ev.samplesStats.UpdatePeak(ev.currentSamples)
		return res, ws
	}

	panic(fmt.Errorf("unhandled expression of type: %T", expr))
}
