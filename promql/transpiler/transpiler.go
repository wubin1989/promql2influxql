package transpiler

import (
	"fmt"
	"github.com/influxdata/influxql"
	"github.com/pkg/errors"
	"github.com/prometheus/prometheus/model/timestamp"
	"github.com/prometheus/prometheus/promql/parser"
	"time"
)

type Transpiler struct {
	// The time boundaries for the translation
	Start, End     *time.Time
	Timezone       *time.Location
	Evaluation     *time.Time
	parenExprCount int
	aggregateLevel int
}

type TranspilerOption func(transpiler *Transpiler)

func WithTimezone(tz *time.Location) TranspilerOption {
	return func(transpiler *Transpiler) {
		transpiler.Timezone = tz
	}
}

func WithEvaluation(evaluation *time.Time) TranspilerOption {
	return func(transpiler *Transpiler) {
		transpiler.Evaluation = evaluation
	}
}

func NewTranspiler(start, end *time.Time, options ...TranspilerOption) Transpiler {
	t := Transpiler{
		Start: start,
		End:   end,
	}
	for _, fn := range options {
		fn(&t)
	}
	return t
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
func (t *Transpiler) Transpile(expr parser.Expr) (string, error) {
	preprocessExprHelper(expr, *t.Start, *t.End)
	parser.Walk(labelNameEscaper{}, expr, nil)

	influxNode, err := t.transpileExpr(expr)
	if err != nil {
		return "", errors.Errorf("error transpiling expression: %s", err)
	}
	return influxNode.String(), nil
}

func handleNodeNotSupported(expr parser.Expr) error {
	return errors.Errorf("PromQL node type %T is not supported yet", expr)
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
	//case *parser.Call:
	//	return t.transpileCall(e)
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

func makeInt64Pointer(val int64) *int64 {
	valp := new(int64)
	*valp = val
	return valp
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
		preprocessExprHelper(n.LHS, start, end)
		preprocessExprHelper(n.RHS, start, end)

		return false

	case *parser.Call:
		for i := range n.Args {
			preprocessExprHelper(n.Args[i], start, end)
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
		preprocessExprHelper(n.Expr, start, end)
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

type Condition influxql.BinaryExpr

func (receiver *Condition) Add(expr influxql.Expr) *Condition {
	return &Condition{
		Op:  influxql.ADD,
		LHS: (*influxql.BinaryExpr)(receiver),
		RHS: expr,
	}
}

func (receiver *Condition) Sub(expr influxql.Expr) *Condition {
	return &Condition{
		Op:  influxql.SUB,
		LHS: (*influxql.BinaryExpr)(receiver),
		RHS: expr,
	}
}

func (receiver *Condition) Mul(expr influxql.Expr) *Condition {
	return &Condition{
		Op:  influxql.MUL,
		LHS: (*influxql.BinaryExpr)(receiver),
		RHS: expr,
	}
}

func (receiver *Condition) Div(expr influxql.Expr) *Condition {
	return &Condition{
		Op:  influxql.DIV,
		LHS: (*influxql.BinaryExpr)(receiver),
		RHS: expr,
	}
}

func (receiver *Condition) Mod(expr influxql.Expr) *Condition {
	return &Condition{
		Op:  influxql.MOD,
		LHS: (*influxql.BinaryExpr)(receiver),
		RHS: expr,
	}
}

func (receiver *Condition) BitwiseAnd(expr influxql.Expr) *Condition {
	return &Condition{
		Op:  influxql.BITWISE_AND,
		LHS: (*influxql.BinaryExpr)(receiver),
		RHS: expr,
	}
}

func (receiver *Condition) BitwiseOr(expr influxql.Expr) *Condition {
	return &Condition{
		Op:  influxql.BITWISE_OR,
		LHS: (*influxql.BinaryExpr)(receiver),
		RHS: expr,
	}
}

func (receiver *Condition) BitwiseXor(expr influxql.Expr) *Condition {
	return &Condition{
		Op:  influxql.BITWISE_XOR,
		LHS: (*influxql.BinaryExpr)(receiver),
		RHS: expr,
	}
}

func (receiver *Condition) And(expr influxql.Expr) *Condition {
	return &Condition{
		Op:  influxql.AND,
		LHS: (*influxql.BinaryExpr)(receiver),
		RHS: expr,
	}
}

func (receiver *Condition) Or(expr influxql.Expr) *Condition {
	return &Condition{
		Op:  influxql.OR,
		LHS: (*influxql.BinaryExpr)(receiver),
		RHS: expr,
	}
}

func (receiver *Condition) Eq(expr influxql.Expr) *Condition {
	return &Condition{
		Op:  influxql.EQ,
		LHS: (*influxql.BinaryExpr)(receiver),
		RHS: expr,
	}
}

func (receiver *Condition) Neq(expr influxql.Expr) *Condition {
	return &Condition{
		Op:  influxql.NEQ,
		LHS: (*influxql.BinaryExpr)(receiver),
		RHS: expr,
	}
}

func (receiver *Condition) Eqregex(expr influxql.Expr) *Condition {
	return &Condition{
		Op:  influxql.EQREGEX,
		LHS: (*influxql.BinaryExpr)(receiver),
		RHS: expr,
	}
}

func (receiver *Condition) Neqregex(expr influxql.Expr) *Condition {
	return &Condition{
		Op:  influxql.NEQREGEX,
		LHS: (*influxql.BinaryExpr)(receiver),
		RHS: expr,
	}
}

func (receiver *Condition) Lt(expr influxql.Expr) *Condition {
	return &Condition{
		Op:  influxql.LT,
		LHS: (*influxql.BinaryExpr)(receiver),
		RHS: expr,
	}
}

func (receiver *Condition) Lte(expr influxql.Expr) *Condition {
	return &Condition{
		Op:  influxql.LTE,
		LHS: (*influxql.BinaryExpr)(receiver),
		RHS: expr,
	}
}

func (receiver *Condition) Gt(expr influxql.Expr) *Condition {
	return &Condition{
		Op:  influxql.GT,
		LHS: (*influxql.BinaryExpr)(receiver),
		RHS: expr,
	}
}

func (receiver *Condition) Gte(expr influxql.Expr) *Condition {
	return &Condition{
		Op:  influxql.GTE,
		LHS: (*influxql.BinaryExpr)(receiver),
		RHS: expr,
	}
}
