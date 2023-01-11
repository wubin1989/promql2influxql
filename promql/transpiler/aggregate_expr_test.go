package transpiler

import (
	"github.com/influxdata/influxql"
	"github.com/prometheus/prometheus/promql/parser"
	"github.com/wubin1989/promql2influxql/command"
	"github.com/wubin1989/promql2influxql/promql/testinghelper"
	"reflect"
	"testing"
	"time"
)

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
				a: testinghelper.AggregateExpr(`topk(3, go_gc_duration_seconds_count)`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, top(last, 3) FROM (SELECT *::tag, last(value) FROM go_gc_duration_seconds_count GROUP BY *)`),
			wantErr: false,
		},
		{
			name: "",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				a: testinghelper.AggregateExpr(`sum(go_gc_duration_seconds_count) by (container)`),
			},
			want:    influxql.MustParseStatement(`SELECT sum(last) FROM (SELECT *::tag, last(value) FROM go_gc_duration_seconds_count GROUP BY *) GROUP BY container`),
			wantErr: false,
		},
		{
			name: "",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				a: testinghelper.AggregateExpr(`sum by (endpoint) (topk(1, go_gc_duration_seconds_count) by (container))`),
			},
			want:    influxql.MustParseStatement(`SELECT sum(top) FROM (SELECT *::tag, top(last, 1) FROM (SELECT *::tag, last(value) FROM go_gc_duration_seconds_count GROUP BY *) GROUP BY container) GROUP BY endpoint`),
			wantErr: false,
		},
		{
			name: "",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				a: testinghelper.AggregateExpr(`sum by (endpoint) (sum(go_gc_duration_seconds_count) by (container))`),
			},
			want:    influxql.MustParseStatement(`SELECT sum(sum) FROM (SELECT sum(last) FROM (SELECT *::tag, last(value) FROM go_gc_duration_seconds_count GROUP BY *) GROUP BY container) GROUP BY endpoint`),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &Transpiler{
				Command: command.Command{
					Start:      tt.fields.Start,
					End:        tt.fields.End,
					Timezone:   tt.fields.Timezone,
					Evaluation: tt.fields.Evaluation,
				},
				timeRange:      0,
				parenExprCount: tt.fields.parenExprCount,
				timeCondition:  nil,
				tagDropped:     false,
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
