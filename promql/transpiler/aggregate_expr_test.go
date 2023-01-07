package transpiler

import (
	"github.com/influxdata/influxql"
	"github.com/prometheus/prometheus/promql/parser"
	"reflect"
	"testing"
	"time"
)

func aggregateExpr(input string) *parser.AggregateExpr {
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

func TestTranspiler_transpileAggregateExpr(t1 *testing.T) {
	type fields struct {
		Start          *time.Time
		End            *time.Time
		Timezone       *time.Location
		Evaluation     *time.Time
		parenExprCount int
	}
	type args struct {
		a *parser.AggregateExpr
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    influxql.Node
		wantErr bool
	}{
		{
			name: "",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				a: aggregateExpr(`topk(3, go_gc_duration_seconds_count) by (container)`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, top(value, 3) FROM go_gc_duration_seconds_count WHERE time < '2023-01-06T07:00:00Z' GROUP BY container`),
			wantErr: false,
		},
		{
			name: "",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				a: aggregateExpr(`sum(go_gc_duration_seconds_count) by (container)`),
			},
			want:    influxql.MustParseStatement(`SELECT sum(value) FROM go_gc_duration_seconds_count WHERE time < '2023-01-06T07:00:00Z' GROUP BY container`),
			wantErr: false,
		},
		{
			name: "",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				a: aggregateExpr(`sum by (endpoint) (topk(1, go_gc_duration_seconds_count) by (container))`),
			},
			want:    influxql.MustParseStatement(`SELECT sum(top) FROM (SELECT *::tag, top(value, 1) FROM go_gc_duration_seconds_count WHERE time < '2023-01-06T07:00:00Z' GROUP BY container) GROUP BY endpoint`),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &Transpiler{
				Start:          tt.fields.Start,
				End:            tt.fields.End,
				Timezone:       tt.fields.Timezone,
				Evaluation:     tt.fields.Evaluation,
				parenExprCount: tt.fields.parenExprCount,
			}
			got, err := t.transpileAggregateExpr(tt.args.a)
			if (err != nil) != tt.wantErr {
				t1.Errorf("transpileAggregateExpr() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got.String(), tt.want.String()) {
				t1.Errorf("transpileAggregateExpr() got = %v, want %v", got, tt.want)
			}
		})
	}
}
