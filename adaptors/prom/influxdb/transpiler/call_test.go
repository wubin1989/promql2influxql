package transpiler

import (
	"github.com/influxdata/influxql"
	"github.com/prometheus/prometheus/promql/parser"
	"github.com/wubin1989/promql2influxql/adaptors/prom/influxdb/testinghelper"
	"github.com/wubin1989/promql2influxql/adaptors/prom/models"
	"reflect"
	"testing"
	"time"
)

func TestTranspiler_transpileCall(t1 *testing.T) {
	type fields struct {
		Start          *time.Time
		End            *time.Time
		Timezone       *time.Location
		Evaluation     *time.Time
		Step           time.Duration
		DataType       models.DataType
		timeRange      time.Duration
		parenExprCount int
		timeCondition  influxql.Expr
		tagDropped     bool
	}
	type args struct {
		a *parser.Call
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
				a: testinghelper.CallExpr(`abs(go_gc_duration_seconds_count)`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, abs(last) FROM (SELECT *::tag, last(value) FROM go_gc_duration_seconds_count GROUP BY *)`),
			wantErr: false,
		},
		{
			name: "",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				a: testinghelper.CallExpr(`quantile_over_time(0.5, go_gc_duration_seconds_count[5m])`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, percentile(value, 0.5) FROM go_gc_duration_seconds_count GROUP BY *`),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &Transpiler{
				PromCommand: models.PromCommand{
					Start:      tt.fields.Start,
					End:        tt.fields.End,
					Timezone:   tt.fields.Timezone,
					Evaluation: tt.fields.Evaluation,
					Step:       tt.fields.Step,
					DataType:   tt.fields.DataType,
				},
				timeRange:      tt.fields.timeRange,
				parenExprCount: tt.fields.parenExprCount,
				timeCondition:  tt.fields.timeCondition,
				tagDropped:     tt.fields.tagDropped,
			}
			got, err := t.transpileCall(tt.args.a)
			if (err != nil) != tt.wantErr {
				t1.Errorf("transpileCall() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got.String(), tt.want.String()) {
				t1.Errorf("transpileCall() got = %v, want %v", got, tt.want)
			}
		})
	}
}
