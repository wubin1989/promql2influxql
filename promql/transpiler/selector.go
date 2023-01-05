package transpiler

import (
	"fmt"
	"github.com/influxdata/flux/ast"
	"github.com/influxdata/influxql"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/promql/parser"
	"regexp"
	"time"
)

var labelMatchOps = map[labels.MatchType]ast.OperatorKind{
	labels.MatchEqual:     ast.EqualOperator,
	labels.MatchNotEqual:  ast.NotEqualOperator,
	labels.MatchRegexp:    ast.RegexpMatchOperator,
	labels.MatchNotRegexp: ast.NotRegexpMatchOperator,
}

func transpileLabelMatchersFn(lms []*labels.Matcher) *ast.FunctionExpression {
	return &ast.FunctionExpression{
		Params: []*ast.Property{
			{
				Key: &ast.Identifier{Name: "r"},
			},
		},
		Body: transpileLabelMatchers(lms),
	}
}

func transpileLabelMatchers(lms []*labels.Matcher) ast.Expression {
	if len(lms) == 0 {
		panic("empty label matchers")
	}
	if len(lms) == 1 {
		return transpileLabelMatcher(lms[0])
	}
	return &ast.LogicalExpression{
		Operator: ast.AndOperator,
		Left:     transpileLabelMatcher(lms[0]),
		// Recurse until we have all label matchers AND-ed together in a right-heavy tree.
		Right: transpileLabelMatchers(lms[1:]),
	}
}

func transpileLabelMatcher(lm *labels.Matcher) *ast.BinaryExpression {
	op, ok := labelMatchOps[lm.Type]
	if !ok {
		panic(fmt.Errorf("invalid label matcher type %v", lm.Type))
	}
	be := &ast.BinaryExpression{
		Operator: op,
		Left:     member("r", lm.Name),
	}
	if op == ast.EqualOperator || op == ast.NotEqualOperator {
		be.Right = &ast.StringLiteral{Value: lm.Value}
	} else {
		// PromQL parsing already validates regexes.
		// PromQL regexes are always full-string matches / fully anchored.
		be.Right = &ast.RegexpLiteral{Value: regexp.MustCompile("^(?:" + lm.Value + ")$")}
	}
	return be
}

var dropMeasurementCall = call(
	"drop",
	map[string]ast.Expression{
		"columns": &ast.ArrayExpression{
			Elements: []ast.Expression{
				&ast.StringLiteral{Value: "_measurement"},
			},
		},
	},
)

func (t *Transpiler) transpileInstantVectorSelector(v *parser.VectorSelector) influxql.Node {
	/**
	Name string
	// OriginalOffset is the actual offset that was set in the query.
	// This never changes.
	OriginalOffset time.Duration
	// Offset is the offset used during the query execution
	// which is calculated using the original offset, at modifier time,
	// eval time, and subquery offsets in the AST tree.
	Offset        time.Duration
	LabelMatchers []*labels.Matcher
	*/

	// TODO time range selector
	// TODO histogram
	// TODO summary

	now := time.Now()
	var start, end *time.Time

	end = &now
	if t.Start != nil || t.End != nil {
		if t.Start != nil {
			start = t.Start
		}
		if t.End != nil {
			end = t.End
		}
	} else if v.Timestamp != nil {
		ts := time.UnixMilli(*v.Timestamp)
		end = &ts
	}
	if start == nil {
		ts := end.Add(-v.OriginalOffset)
		end = &ts
	}

	binaryExpr := &influxql.BinaryExpr{
		Op: influxql.LT,
		LHS: &influxql.VarRef{
			Val: "time",
		},
		RHS: &influxql.TimeLiteral{
			Val: *end,
		},
	}

	if start != nil || len(v.LabelMatchers) > 0 {
		condition := (*Condition)(binaryExpr)
		condition = condition.And(&influxql.BinaryExpr{
			Op: influxql.GTE,
			LHS: &influxql.VarRef{
				Val: "time",
			},
			RHS: &influxql.TimeLiteral{
				Val: *start,
			},
		})
		for _, item := range v.LabelMatchers {
			// TODO
		}

	}

	selectStatement := influxql.SelectStatement{
		Fields: []*influxql.Field{
			{Expr: &influxql.Wildcard{}},
		},
		Dimensions: nil,
		Sources:    []influxql.Source{&influxql.Measurement{Name: v.Name}},
		Condition:  nil,
		SortFields: nil,
		Limit:      0,
		Offset:     0,
		SLimit:     0,
		SOffset:    0,
		Fill:       0,
		FillValue:  nil,
		Location:   nil,
		TimeAlias:  "",
		OmitTime:   false,
		StripName:  false,
		EmitName:   "",
		Dedupe:     false,
	}

	return &selectStatement
}

func (t *Transpiler) transpileRangeVectorSelector(v *parser.MatrixSelector) *ast.PipeExpression {
	var windowCall *ast.CallExpression
	var windowFilterCall *ast.CallExpression
	if t.Resolution > 0 {
		// For range queries:
		// At every resolution step, include the specified range of data.
		windowCall = call("window", map[string]ast.Expression{
			"every":  &ast.DurationLiteral{Values: []ast.Duration{{Magnitude: t.Resolution.Nanoseconds(), Unit: "ns"}}},
			"period": &ast.DurationLiteral{Values: []ast.Duration{{Magnitude: v.Range.Nanoseconds(), Unit: "ns"}}},
			"offset": &ast.DurationLiteral{Values: []ast.Duration{{Magnitude: t.Start.UnixNano() % t.Resolution.Nanoseconds(), Unit: "ns"}}},
		})

		// Remove any windows smaller than the specified range at the edges of the graph range.
		windowFilterCall = call("filter", map[string]ast.Expression{"fn": windowCutoffFn(t.Start.Add(-v.Offset), t.End.Add(-v.Range-v.Offset))})
	}

	return buildPipeline(
		// Select all Prometheus data.
		call("from", map[string]ast.Expression{"bucket": &ast.StringLiteral{Value: t.Bucket}}),
		// Query entire graph range.
		call("range", map[string]ast.Expression{
			"start": &ast.DateTimeLiteral{Value: t.Start.Add(-v.Range - v.Offset)},
			"stop":  &ast.DateTimeLiteral{Value: t.End.Add(-v.Offset)},
		}),
		// Apply label matching filters.
		call("filter", map[string]ast.Expression{"fn": transpileLabelMatchersFn(v.LabelMatchers)}),
		windowCall,
		windowFilterCall,
		// Apply offsets to make past data look like it's in the present.
		call("timeShift", map[string]ast.Expression{
			"duration": &ast.DurationLiteral{Values: []ast.Duration{{Magnitude: v.Offset.Nanoseconds(), Unit: "ns"}}},
		}),
		dropMeasurementCall,
	)
}
