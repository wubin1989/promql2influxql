package transpiler

import (
	"github.com/influxdata/influxql"
	"github.com/pkg/errors"
	"github.com/prometheus/prometheus/promql/parser"
)

func (t *Transpiler) transpileUnaryExpr(ue *parser.UnaryExpr) (influxql.Node, error) {
	node, err := t.transpileExpr(ue.Expr)
	if err != nil {
		return nil, errors.Errorf("error transpiling expression in unary expression: %s", err)
	}

	switch ue.Op {
	case parser.ADD, parser.SUB:
		mul := 1
		if ue.Op == parser.SUB {
			mul = -1
		}
		switch n := node.(type) {
		case influxql.Expr:
			switch expr := n.(type) {
			case *influxql.NumberLiteral:
				expr.Val *= float64(mul)
			case *influxql.IntegerLiteral:
				expr.Val *= int64(mul)
			default:
				return &influxql.BinaryExpr{
					Op: influxql.MUL,
					LHS: &influxql.IntegerLiteral{
						Val: int64(mul),
					},
					RHS: expr,
				}, nil
			}
		case influxql.Statement:
			switch statement := n.(type) {
			case *influxql.SelectStatement:
				if ue.Op == parser.ADD {
					return node, nil
				}
				statement.Fields = []*influxql.Field{
					{
						Expr: &influxql.Wildcard{
							Type: influxql.TAG,
						},
					},
					{
						Expr: &influxql.BinaryExpr{
							Op: influxql.MUL,
							LHS: &influxql.IntegerLiteral{
								Val: int64(mul),
							},
							RHS: &influxql.VarRef{
								Val: "value",
							},
						},
					},
				}
			default:

			}
		}
		return node, nil
	default:
		// PromQL fails to parse unary operators other than +/-, so this should never happen.
		return nil, errors.Errorf("invalid unary expression operator type (this should never happen)")
	}
}
