package evaluator

import (
	"github.com/influxdata/influxql"
	"github.com/prometheus/prometheus/promql/parser"
	"math"
)

// Evaluator is used for evaluating PromQL expression locally
type Evaluator struct {
}

// EvalYieldsFloatExpr evaluates PromQL expression and returns an InfluxQL NumberLiteral
func (receiver *Evaluator) EvalYieldsFloatExpr(expr parser.Expr) *influxql.NumberLiteral {
	switch v := expr.(type) {
	case *parser.NumberLiteral:
		return &influxql.NumberLiteral{
			Val: v.Val,
		}
	case *parser.BinaryExpr:
		lhs := receiver.EvalYieldsFloatExpr(v.LHS)
		rhs := receiver.EvalYieldsFloatExpr(v.RHS)
		switch v.Op {
		case parser.ADD:
			return &influxql.NumberLiteral{
				Val: lhs.Val + rhs.Val,
			}
		case parser.SUB:
			return &influxql.NumberLiteral{
				Val: lhs.Val - rhs.Val,
			}
		case parser.MUL:
			return &influxql.NumberLiteral{
				Val: lhs.Val * rhs.Val,
			}
		case parser.DIV:
			return &influxql.NumberLiteral{
				Val: lhs.Val / rhs.Val,
			}
		case parser.MOD:
			return &influxql.NumberLiteral{
				Val: math.Mod(lhs.Val, rhs.Val),
			}
		case parser.POW:
			return &influxql.NumberLiteral{
				Val: math.Pow(lhs.Val, rhs.Val),
			}
		default:
			return &influxql.NumberLiteral{
				Val: 0,
			}
		}
	case *parser.UnaryExpr:
		result := receiver.EvalYieldsFloatExpr(v.Expr)
		switch v.Op {
		case parser.ADD:
			return result
		case parser.SUB:
			return &influxql.NumberLiteral{
				Val: -1 * result.Val,
			}
		default:
			return &influxql.NumberLiteral{
				Val: 0,
			}
		}
	case *parser.ParenExpr:
		return receiver.EvalYieldsFloatExpr(v.Expr)
	default:
		return &influxql.NumberLiteral{
			Val: 0,
		}
	}
}
