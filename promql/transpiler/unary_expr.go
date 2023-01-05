package transpiler

import (
	"fmt"
	"github.com/influxdata/influxql"
	"github.com/prometheus/prometheus/promql/parser"
)

func (t *Transpiler) transpileUnaryExpr(ue *parser.UnaryExpr) (influxql.Expr, error) {
	expr, err := t.transpileExpr(ue.Expr)
	if err != nil {
		return nil, fmt.Errorf("error transpiling expression in unary expression: %s", err)
	}

	switch ue.Op {
	case parser.ADD, parser.SUB:
		mul := 1
		if ue.Op == parser.SUB {
			mul = -1
		}
		switch lit := expr.(type) {
		case *influxql.NumberLiteral:
			lit.Val *= float64(mul)
		default:
		}
		return expr, nil
	default:
		// PromQL fails to parse unary operators other than +/-, so this should never happen.
		return nil, fmt.Errorf("invalid unary expression operator type (this should never happen)")
	}
}
