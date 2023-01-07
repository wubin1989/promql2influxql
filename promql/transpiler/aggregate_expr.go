package transpiler

import (
	"github.com/influxdata/influxql"
	"github.com/pkg/errors"
	"github.com/prometheus/prometheus/promql/parser"

	"github.com/prometheus/common/model"
)

// Taken from Prometheus.
const (
	// The largest SampleValue that can be converted to an int64 without overflow.
	maxInt64 = 9223372036854774784
	// The smallest SampleValue that can be converted to an int64 without underflow.
	minInt64 = -9223372036854775808
)

type aggregateFn struct {
	name      string
	dropField bool
	// All PromQL aggregation operators drop non-grouping labels, but some
	// of the (non-aggregation) Flux counterparts don't. This field indicates
	// that the non-grouping drop needs to be explicitly added to the pipeline.
	dropNonGrouping bool
}

var aggregateFns = map[parser.ItemType]aggregateFn{
	parser.SUM:     {name: "sum", dropField: true, dropNonGrouping: false},
	parser.AVG:     {name: "mean", dropField: true, dropNonGrouping: false},
	parser.MAX:     {name: "max", dropField: true, dropNonGrouping: true},
	parser.MIN:     {name: "min", dropField: true, dropNonGrouping: true},
	parser.COUNT:   {name: "count", dropField: true, dropNonGrouping: false},
	parser.STDDEV:  {name: "stddev", dropField: true, dropNonGrouping: false},
	parser.TOPK:    {name: "top", dropField: false, dropNonGrouping: false},
	parser.BOTTOMK: {name: "bottom", dropField: false, dropNonGrouping: false},
}

func dropNonGroupingColsCall(groupCols []string, without bool) *influxql.CallExpression {
	if without {
		cols := make([]string, 0, len(groupCols))
		// Remove "_value" and "_stop" from list of columns to drop.
		for _, col := range groupCols {
			if col != "_value" && col != "_stop" {
				cols = append(cols, col)
			}
		}

		return call("drop", map[string]influxql.Expression{"columns": columnList(cols...)})
	}

	// We want to keep value and stop columns even if they are not explicitly in the grouping labels.
	cols := append(groupCols, "_value", "_stop")
	// TODO: This errors with non-existent columns. In PromQL, this is a no-op.
	// Blocked on https://github.com/influxdata/flux/issues/1118.
	return call("keep", map[string]influxql.Expression{"columns": columnList(cols...)})
}

func (t *Transpiler) transpileAggregateExpr(a *parser.AggregateExpr) (influxql.Node, error) {
	expr, err := t.transpileExpr(a.Expr)
	if err != nil {
		return nil, errors.Errorf("error transpiling aggregate sub-expression: %s", err)
	}

	aggFn, ok := aggregateFns[a.Op]
	if !ok {
		return nil, errors.Errorf("unsupported aggregation type %s", a.Op)
	}

	groupCols := columnList(a.Grouping...)
	aggArgs := map[string]influxql.Expression{}

	switch a.Op {
	case parser.TOPK, parser.BOTTOMK:
		n, ok := a.Param.(*parser.NumberLiteral)
		if !ok {
			return nil, errors.Errorf("arbitrary scalar subexpressions not supported yet")
		}
		if n.Val > maxInt64 || n.Val < minInt64 {
			return nil, errors.Errorf("scalar value %v overflows int64", n)
		}
		aggArgs["n"] = &influxql.IntegerLiteral{Value: int64(n.Val)}

	case parser.Quantile:
		// TODO: Allow any constant scalars here.
		// The PromQL parser already verifies that a.Param is a scalar.
		n, ok := a.Param.(*parser.NumberLiteral)
		if !ok {
			return nil, errors.Errorf("arbitrary scalar subexpressions not supported yet")
		}
		aggArgs["q"] = &influxql.FloatLiteral{Value: n.Val}
		aggArgs["method"] = &influxql.StringLiteral{Value: "exact_mean"}

	case parser.Stddev, parser.Stdvar:
		aggArgs["mode"] = &influxql.StringLiteral{Value: "population"}
	}

	mode := "by"
	dropField := true
	if a.Without {
		mode = "except"
		groupCols.Elements = append(
			groupCols.Elements,
			// "_time" is not always present, but if it is, we don't want to group by it.
			&influxql.StringLiteral{Value: "_time"},
			&influxql.StringLiteral{Value: "_value"},
		)
	} else {
		groupCols.Elements = append(
			groupCols.Elements,
			&influxql.StringLiteral{Value: "_start"},
			&influxql.StringLiteral{Value: "_stop"},
		)
		for _, col := range a.Grouping {
			if col == model.MetricNameLabel {
				dropField = false
			}
		}
	}

	pipeline := buildPipeline(
		// Get the underlying data.
		expr,
		// Group values according to by() / without() clauses.
		call("group", map[string]influxql.Expression{
			"columns": groupCols,
			"mode":    &influxql.StringLiteral{Value: mode},
		}),
		// Aggregate.
		call(aggFn.name, aggArgs),
	)
	if aggFn.name == "count" {
		pipeline = buildPipeline(pipeline, call("toFloat", nil))
	}
	if aggFn.dropNonGrouping {
		// Drop labels that are not part of the grouping.
		pipeline = buildPipeline(pipeline, dropNonGroupingColsCall(a.Grouping, a.Without))
	}
	if aggFn.dropField && dropField {
		pipeline = buildPipeline(
			pipeline,
			dropFieldAndTimeCall,
		)
	}
	if a.Op == parser.Stdvar {
		pipeline = buildPipeline(
			pipeline,
			call("map", map[string]influxql.Expression{
				"fn": scalarArithBinaryMathFn("pow", &influxql.FloatLiteral{Value: 2}, false),
			}),
		)
	}
	return pipeline, nil
}
