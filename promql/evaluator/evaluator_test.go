package evaluator

import (
	"github.com/prometheus/prometheus/promql/parser"
	"testing"
)

func binaryExpr(input string) *parser.BinaryExpr {
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

func TestEvaluator_EvalYieldsFloatExpr(t *testing.T) {
	type args struct {
		expr parser.Expr
	}
	tests := []struct {
		name string
		args args
		want float64
	}{
		{
			name: "",
			args: args{
				expr: binaryExpr("3 + (2*2)"),
			},
			want: 7,
		},
		{
			name: "",
			args: args{
				expr: binaryExpr("5 % 2"),
			},
			want: 1,
		},
		{
			name: "",
			args: args{
				expr: binaryExpr("4/2"),
			},
			want: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receiver := &Evaluator{}
			if got := receiver.EvalYieldsFloatExpr(tt.args.expr); got.Val != tt.want {
				t.Errorf("EvalYieldsFloatExpr() = %v, want %v", got, tt.want)
			}
		})
	}
}
