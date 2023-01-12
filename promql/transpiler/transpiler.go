package transpiler

import (
	"github.com/influxdata/influxql"
	"github.com/pkg/errors"
	"github.com/prometheus/prometheus/promql/parser"
	"github.com/wubin1989/promql2influxql/command"
	"reflect"
	"time"
)

const (
	defaultValueFieldKey = "value"
)

type Transpiler struct {
	command.Command
	timeRange      time.Duration
	parenExprCount int
	timeCondition  influxql.Expr
	tagDropped     bool
}

// Transpile converts a PromQL expression with the time ranges set in the transpiler
// into a Flux file. The resulting Flux file can be executed and the result needs to
// be transformed using FluxResultToPromQLValue() (implemented in the InfluxDB repo)
// to get a result value that is fully equivalent to the result of a native PromQL
// execution.
//
// During the transpilation, the transpiler recursively translates the PromQL AST into
// equivalent Flux nodes. Each PromQL node translates into one or more Flux
// constructs that as a group (corresponding to the PromQL node) have to
// keep the following invariants:
//
// - The "_field" column contains the PromQL metric name, if any.
// - The "_measurement" column is ignored (always set to constant "prometheus").
// - The "_time" column contains the sample timestamp as long as a raw sample has been
//   selected from storage and not processed further. Otherwise, "_time" will be
//   empty.
// - The "_stop" column contains the stop timestamp of windows that are equivalent to
//   the resolution steps in parser. If "_time" is no longer present, "_stop" becomes
//   the output timestamp for a sample.
// - The "_value" column is always of float type and represents the PromQL sample value.
// - Other columns map to PromQL label names, with escaping applied ("_foo" -> "~_foo").
// - Tables should be grouped by all columns except for "_time" and "_value". Each Flux
//   table represents one PromQL series, with potentially multiple samples over time.
func (t *Transpiler) Transpile(expr parser.Expr) (influxql.Node, error) {
	influxNode, err := t.transpile(expr)
	if err != nil {
		return nil, errors.Errorf("error transpiling expression: %s", err)
	}
	return influxNode, nil
}

func handleNodeNotSupported(expr parser.Expr) error {
	return errors.Errorf("PromQL node type %T is not supported yet", expr)
}

func (t *Transpiler) setTimeCondition(node influxql.Statement) {
	switch statement := node.(type) {
	case *influxql.SelectStatement, *influxql.ShowTagValuesStatement:
		conditionValue := reflect.ValueOf(statement).Elem().FieldByName("Condition")
		if conditionValue.IsValid() {
			if !conditionValue.IsNil() {
				conditionValue.Set(reflect.ValueOf(&influxql.BinaryExpr{
					Op:  influxql.AND,
					LHS: t.timeCondition,
					RHS: conditionValue.Interface().(influxql.Expr),
				}))
			} else {
				conditionValue.Set(reflect.ValueOf(t.timeCondition))
			}
		}
		locationValue := reflect.ValueOf(statement).Elem().FieldByName("Location")
		if locationValue.IsValid() {
			locationValue.Set(reflect.ValueOf(t.Timezone))
		}
	}
}

func (t *Transpiler) transpile(expr parser.Expr) (influxql.Node, error) {
	if t.Start != nil {
		if expr.Type() != parser.ValueTypeVector && expr.Type() != parser.ValueTypeScalar {
			return nil, errors.Errorf("invalid expression type %q for range query, must be Scalar or instant Vector", parser.DocumentedType(expr.Type()))
		}
	}
	node, err := t.transpileExpr(expr)
	if err != nil {
		return nil, errors.Errorf("error transpiling expression: %s", err)
	}
	switch n := node.(type) {
	case influxql.Statement:
		switch statement := n.(type) {
		case *influxql.SelectStatement:
			if t.DataType == command.GRAPH_DATA {
				var timeRange time.Duration
				if t.timeRange > 0 {
					timeRange = t.timeRange
				} else {
					timeRange = t.Step
				}
				statement.Dimensions = append(statement.Dimensions, &influxql.Dimension{
					Expr: &influxql.Call{
						Name: "time",
						Args: []influxql.Expr{
							&influxql.DurationLiteral{Val: timeRange},
						},
					},
				})
			}
		}
		t.setTimeCondition(n)
	}
	return node, nil
}

func (t *Transpiler) transpileExpr(expr parser.Expr) (influxql.Node, error) {
	switch e := expr.(type) {
	case *parser.ParenExpr:
		t.parenExprCount++
		return t.transpileExpr(e.Expr)
	case *parser.UnaryExpr:
		return t.transpileUnaryExpr(e)
	case *parser.NumberLiteral:
		return &influxql.NumberLiteral{Val: e.Val}, nil
	case *parser.StringLiteral:
		return &influxql.StringLiteral{Val: e.Val}, nil
	case *parser.VectorSelector:
		return t.transpileInstantVectorSelector(e)
	case *parser.MatrixSelector:
		return t.transpileRangeVectorSelector(e)
	case *parser.AggregateExpr:
		return t.transpileAggregateExpr(e)
	case *parser.BinaryExpr:
		return t.transpileBinaryExpr(e)
	case *parser.Call:
		return t.transpileCall(e)
	case *parser.SubqueryExpr:
		return nil, handleNodeNotSupported(expr)
	default:
		return nil, handleNodeNotSupported(expr)
	}
}

func yieldsTable(expr parser.Expr) bool {
	return !yieldsFloat(expr)
}

func yieldsFloat(expr parser.Expr) bool {
	switch v := expr.(type) {
	case *parser.NumberLiteral:
		return true
	case *parser.BinaryExpr:
		return yieldsFloat(v.LHS) && yieldsFloat(v.RHS)
	case *parser.UnaryExpr:
		return yieldsFloat(v.Expr)
	case *parser.ParenExpr:
		return yieldsFloat(v.Expr)
	default:
		return false
	}
}

var YieldsFloat = yieldsFloat

func makeInt64Pointer(val int64) *int64 {
	valp := new(int64)
	*valp = val
	return valp
}

type Condition influxql.BinaryExpr

func (receiver *Condition) And(expr *influxql.BinaryExpr) *Condition {
	return &Condition{
		Op:  influxql.AND,
		LHS: (*influxql.BinaryExpr)(receiver),
		RHS: expr,
	}
}

func (receiver *Condition) Or(expr *influxql.BinaryExpr) *Condition {
	return &Condition{
		Op:  influxql.OR,
		LHS: (*influxql.BinaryExpr)(receiver),
		RHS: expr,
	}
}
