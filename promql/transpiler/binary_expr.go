package transpiler

import (
	"github.com/influxdata/influxql"
	"github.com/pkg/errors"
	"github.com/prometheus/prometheus/promql/parser"
)

const (
	POW = "pow"
)

var arithBinOps = map[parser.ItemType]influxql.Token{
	parser.ADD: influxql.ADD,
	parser.SUB: influxql.SUB,
	parser.MUL: influxql.MUL,
	parser.DIV: influxql.DIV,
	parser.MOD: influxql.MOD,
}

var arithBinOpFns = map[parser.ItemType]string{
	parser.POW: POW,
}

var compBinOps = map[parser.ItemType]influxql.Token{
	parser.EQL: influxql.EQ,
	parser.NEQ: influxql.NEQ,
	parser.GTR: influxql.GT,
	parser.LSS: influxql.LT,
	parser.GTE: influxql.GTE,
	parser.LTE: influxql.LTE,
}

const (
	LEFT_EXPR = iota + 1
	RIGHT_EXPR
)

func (t *Transpiler) NewBinaryExpr(op influxql.Token, lhs, rhs influxql.Expr) influxql.Expr {
	expr := &influxql.BinaryExpr{
		Op:  op,
		LHS: lhs,
		RHS: rhs,
	}
	if t.parenExprCount > 0 {
		defer func() {
			t.parenExprCount--
		}()
		return &influxql.ParenExpr{
			Expr: expr,
		}
	}
	return expr
}

func (t *Transpiler) NewBinaryCallExpr(opFn string, lhs, rhs influxql.Expr) influxql.Expr {
	expr := &influxql.Call{
		Name: opFn,
		Args: []influxql.Expr{
			lhs,
			rhs,
		},
	}
	if t.parenExprCount > 0 {
		defer func() {
			t.parenExprCount--
		}()
	}
	return expr
}

func (t *Transpiler) transpileArithBin(b *parser.BinaryExpr, op influxql.Token, lhs, rhs influxql.Node) (influxql.Node, error) {
	m := make(map[influxql.Node]int)
	table := lhs
	parameter := rhs
	m[table] = LEFT_EXPR
	m[parameter] = RIGHT_EXPR
	switch {
	case yieldsFloat(b.LHS) && yieldsTable(b.RHS):
		table = rhs
		parameter = lhs
		m[table] = RIGHT_EXPR
		m[parameter] = LEFT_EXPR
	}
	switch n := table.(type) {
	case influxql.Expr:
		return t.NewBinaryExpr(op, lhs.(influxql.Expr), rhs.(influxql.Expr)), nil
	case influxql.Statement:
		switch statement := n.(type) {
		case *influxql.SelectStatement:
			field := statement.Fields[len(statement.Fields)-1]
			var left, right influxql.Expr
			switch m[table] {
			case LEFT_EXPR:
				left = field.Expr
				right = parameter.(influxql.Expr)
			case RIGHT_EXPR:
				left = parameter.(influxql.Expr)
				right = field.Expr
			default:
			}
			statement.Fields[len(statement.Fields)-1] = &influxql.Field{
				Expr: t.NewBinaryExpr(op, left, right),
			}
		default:
			return nil, ErrPromExprNotSupported
		}
	}
	return table, nil
}

func (t *Transpiler) transpileArithBinFns(b *parser.BinaryExpr, opFn string, lhs, rhs influxql.Node) (influxql.Node, error) {
	m := make(map[influxql.Node]int)
	table := lhs
	parameter := rhs
	m[table] = LEFT_EXPR
	m[parameter] = RIGHT_EXPR
	switch {
	case yieldsFloat(b.LHS) && yieldsTable(b.RHS):
		table = rhs
		parameter = lhs
		m[table] = RIGHT_EXPR
		m[parameter] = LEFT_EXPR
	}
	switch n := table.(type) {
	case influxql.Expr:
		return t.NewBinaryCallExpr(opFn, lhs.(influxql.Expr), rhs.(influxql.Expr)), nil
	case influxql.Statement:
		switch statement := n.(type) {
		case *influxql.SelectStatement:
			switch opFn {
			case POW:
				field := statement.Fields[len(statement.Fields)-1]
				var left, right influxql.Expr
				switch m[table] {
				case LEFT_EXPR:
					left = field.Expr
					right = parameter.(influxql.Expr)
				case RIGHT_EXPR:
					left = parameter.(influxql.Expr)
					right = field.Expr
				default:
				}
				statement.Fields[len(statement.Fields)-1] = &influxql.Field{
					Expr: t.NewBinaryCallExpr(opFn, left, right),
				}
			default:
				return nil, ErrPromExprNotSupported
			}
		default:
			return nil, ErrPromExprNotSupported
		}
	}
	return table, nil
}

func (t *Transpiler) transpileCompBinOps(b *parser.BinaryExpr, op influxql.Token, lhs, rhs influxql.Node) (influxql.Node, error) {
	m := make(map[influxql.Node]int)
	table := lhs
	parameter := rhs
	m[table] = LEFT_EXPR
	m[parameter] = RIGHT_EXPR
	switch {
	case yieldsFloat(b.LHS) && yieldsTable(b.RHS):
		table = rhs
		parameter = lhs
		m[table] = RIGHT_EXPR
		m[parameter] = LEFT_EXPR
	}
	switch n := table.(type) {
	case influxql.Expr:
		return t.NewBinaryExpr(op, lhs.(influxql.Expr), rhs.(influxql.Expr)), nil
	case influxql.Statement:
		switch statement := n.(type) {
		case *influxql.SelectStatement:
			var selectStatement influxql.SelectStatement
			selectStatement.Sources = []influxql.Source{
				&influxql.SubQuery{
					Statement: statement,
				},
			}
			if !t.tagDropped {
				selectStatement.Fields = append(selectStatement.Fields, &influxql.Field{
					Expr: &influxql.Wildcard{
						Type: influxql.TAG,
					},
				})
			}
			field := &influxql.Field{
				Expr: &influxql.VarRef{
					Val: statement.Fields[len(statement.Fields)-1].Name(),
				},
			}
			selectStatement.Fields = append(selectStatement.Fields, field)
			var left, right influxql.Expr
			switch m[table] {
			case LEFT_EXPR:
				left = field.Expr
				right = parameter.(influxql.Expr)
			case RIGHT_EXPR:
				left = parameter.(influxql.Expr)
				right = field.Expr
			default:
			}
			selectStatement.Condition = &influxql.BinaryExpr{
				Op:  op,
				LHS: left,
				RHS: right,
			}
			return &selectStatement, nil
		default:
			return nil, ErrPromExprNotSupported
		}
	default:
		return nil, ErrPromExprNotSupported
	}
}

func (t *Transpiler) transpileBinaryExpr(b *parser.BinaryExpr) (influxql.Node, error) {
	lhs, err := t.transpileExpr(b.LHS)
	if err != nil {
		return nil, errors.Errorf("unable to transpile left-hand side of binary operation: %s", err)
	}
	rhs, err := t.transpileExpr(b.RHS)
	if err != nil {
		return nil, errors.Errorf("unable to transpile right-hand side of binary operation: %s", err)
	}
	switch {
	case yieldsFloat(b.LHS) && yieldsFloat(b.RHS):
		if op, ok := arithBinOps[b.Op]; ok {
			return t.NewBinaryExpr(op, lhs.(influxql.Expr), rhs.(influxql.Expr)), nil
		}

		if opFn, ok := arithBinOpFns[b.Op]; ok {
			return t.NewBinaryCallExpr(opFn, lhs.(influxql.Expr), rhs.(influxql.Expr)), nil
		}

		if op, ok := compBinOps[b.Op]; ok {
			if !b.ReturnBool {
				// This is already caught by the PromQL parser.
				return nil, errors.Errorf("scalar-to-scalar binary op is missing 'bool' modifier (this should never happen)")
			}
			return t.NewBinaryExpr(op, lhs.(influxql.Expr), rhs.(influxql.Expr)), nil
		}

		return nil, errors.Errorf("invalid scalar-scalar binary op %q (this should never happen)", b.Op)
	case yieldsFloat(b.LHS) && yieldsTable(b.RHS), yieldsTable(b.LHS) && yieldsFloat(b.RHS):
		if op, ok := arithBinOps[b.Op]; ok {
			return t.transpileArithBin(b, op, lhs, rhs)
		}

		if opFn, ok := arithBinOpFns[b.Op]; ok {
			return t.transpileArithBinFns(b, opFn, lhs, rhs)
		}

		if op, ok := compBinOps[b.Op]; ok {
			return t.transpileCompBinOps(b, op, lhs, rhs)
		}

		return nil, errors.Errorf("invalid scalar-vector binary op %q (this should never happen)", b.Op)
	default:
		return nil, errors.Errorf("not suppport both sides have VectorSelector expression: %s", b)
	}
}
