package evaluator

import (
	"github.com/prometheus/prometheus/promql/parser"
	"github.com/wubin1989/promql2influxql/promql/testinghelper"
	"testing"
)

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
				expr: testinghelper.BinaryExpr("3 + (2*2)"),
			},
			want: 7,
		},
		{
			name: "",
			args: args{
				expr: testinghelper.BinaryExpr("5 % 2"),
			},
			want: 1,
		},
		{
			name: "",
			args: args{
				expr: testinghelper.BinaryExpr("4/2"),
			},
			want: 2,
		},
		{
			name: "",
			args: args{
				expr: testinghelper.BinaryExpr("4^2"),
			},
			want: 16,
		},
		{
			name: "",
			args: args{
				expr: testinghelper.UnaryExpr("-(3*4)"),
			},
			want: -12,
		},
		{
			name: "",
			args: args{
				expr: testinghelper.UnaryExpr("+(3*4)"),
			},
			want: 12,
		},
		{
			name: "",
			args: args{
				expr: testinghelper.AggregateExpr("sum(cpu)"),
			},
			want: 0,
		},
		{
			name: "",
			args: args{
				expr: testinghelper.BinaryExpr("10-2"),
			},
			want: 8,
		},
		{
			name: "",
			args: args{
				expr: testinghelper.BinaryExpr("10>bool2"),
			},
			want: 0,
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
