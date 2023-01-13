// Package testinghelper only for testing purpose
package testinghelper

import "github.com/prometheus/prometheus/promql/parser"

func UnaryExpr(input string) *parser.UnaryExpr {
	expr, err := parser.ParseExpr(input)
	if err != nil {
		panic(err)
	}
	v, ok := expr.(*parser.UnaryExpr)
	if !ok {
		panic("bad input")
	}
	return v
}

func BinaryExpr(input string) *parser.BinaryExpr {
	expr, err := parser.ParseExpr(input)
	if err != nil {
		panic(err)
	}
	v, ok := expr.(*parser.BinaryExpr)
	if !ok {
		panic("bad input")
	}
	return v
}

func AggregateExpr(input string) *parser.AggregateExpr {
	expr, err := parser.ParseExpr(input)
	if err != nil {
		panic(err)
	}
	v, ok := expr.(*parser.AggregateExpr)
	if !ok {
		panic("bad input")
	}
	return v
}

func CallExpr(input string) *parser.Call {
	expr, err := parser.ParseExpr(input)
	if err != nil {
		panic(err)
	}
	v, ok := expr.(*parser.Call)
	if !ok {
		panic("bad input")
	}
	return v
}

func SubqueryExpr(input string) *parser.SubqueryExpr {
	expr, err := parser.ParseExpr(input)
	if err != nil {
		panic(err)
	}
	v, ok := expr.(*parser.SubqueryExpr)
	if !ok {
		panic("bad input")
	}
	return v
}

func StringLiteralExpr(input string) *parser.StringLiteral {
	expr, err := parser.ParseExpr(input)
	if err != nil {
		panic(err)
	}
	v, ok := expr.(*parser.StringLiteral)
	if !ok {
		panic("bad input")
	}
	return v
}

func VectorSelector(input string) *parser.VectorSelector {
	expr, err := parser.ParseExpr(input)
	if err != nil {
		panic(err)
	}
	v, ok := expr.(*parser.VectorSelector)
	if !ok {
		panic("bad input")
	}
	return v
}

func MatrixSelector(input string) *parser.MatrixSelector {
	expr, err := parser.ParseExpr(input)
	if err != nil {
		panic(err)
	}
	v, ok := expr.(*parser.MatrixSelector)
	if !ok {
		panic("bad input")
	}
	return v
}
