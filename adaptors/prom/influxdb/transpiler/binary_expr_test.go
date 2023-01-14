package transpiler

import (
	"github.com/influxdata/influxql"
	"github.com/prometheus/prometheus/promql/parser"
	"github.com/wubin1989/promql2influxql/adaptors/prom/influxdb/testinghelper"
	"github.com/wubin1989/promql2influxql/applications"
	"reflect"
	"testing"
	"time"
)

func TestTranspiler_transpileBinaryExpr(t1 *testing.T) {
	type fields struct {
		Start      *time.Time
		End        *time.Time
		Timezone   *time.Location
		Evaluation *time.Time
	}
	type args struct {
		b *parser.BinaryExpr
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    influxql.Node
		wantErr bool
	}{
		{
			name: "1",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				b: testinghelper.BinaryExpr(`5 * go_gc_duration_seconds_count`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, 5.000 * last(value) FROM go_gc_duration_seconds_count GROUP BY *`),
			wantErr: false,
		},
		{
			name: "",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				b: testinghelper.BinaryExpr(`5 * rate(go_gc_duration_seconds_count[1m])`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, 5.000 * non_negative_derivative(value) FROM go_gc_duration_seconds_count GROUP BY *`),
			wantErr: false,
		},
		{
			name: "2",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				b: testinghelper.BinaryExpr(`5 * 6 * go_gc_duration_seconds_count`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, 5.000 * 6.000 * last(value) FROM go_gc_duration_seconds_count GROUP BY *`),
			wantErr: false,
		},
		{
			name: "3",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				b: testinghelper.BinaryExpr(`5 * (go_gc_duration_seconds_count - 6)`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, 5.000 * (last(value) - 6.000) FROM go_gc_duration_seconds_count GROUP BY *`),
			wantErr: false,
		},
		{
			name: "4",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				b: testinghelper.BinaryExpr(`(5 * go_gc_duration_seconds_count) - 6`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, (5.000 * last(value)) - 6.000 FROM go_gc_duration_seconds_count GROUP BY *`),
			wantErr: false,
		},
		{
			name: "5",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				b: testinghelper.BinaryExpr(`5 > go_gc_duration_seconds_count`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, last FROM (SELECT *::tag, last(value) FROM go_gc_duration_seconds_count GROUP BY *) WHERE 5.000 > last`),
			wantErr: false,
		},
		{
			name: "6",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				b: testinghelper.BinaryExpr(`go_gc_duration_seconds_count^3`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, pow(last(value), 3.000) FROM go_gc_duration_seconds_count GROUP BY *`),
			wantErr: false,
		},
		{
			name: "7",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				b: testinghelper.BinaryExpr(`go_gc_duration_seconds_count^3^4`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, pow(last(value), pow(3.000, 4.000)) FROM go_gc_duration_seconds_count GROUP BY *`),
			wantErr: false,
		},
		{
			name: "8",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				b: testinghelper.BinaryExpr(`go_gc_duration_seconds_count^(3^4)`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, pow(last(value), pow(3.000, 4.000)) FROM go_gc_duration_seconds_count GROUP BY *`),
			wantErr: false,
		},
		{
			name: "9",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				b: testinghelper.BinaryExpr(`(go_gc_duration_seconds_count^3)^4`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, pow(pow(last(value), 3.000), 4.000) FROM go_gc_duration_seconds_count GROUP BY *`),
			wantErr: false,
		},
		{
			name: "10",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				b: testinghelper.BinaryExpr(`4^go_gc_duration_seconds_count`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, pow(4.000, last(value)) FROM go_gc_duration_seconds_count GROUP BY *`),
			wantErr: false,
		},
		{
			name: "11",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				b: testinghelper.BinaryExpr(`go_gc_duration_seconds_count>=3<4`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, last FROM (SELECT *::tag, last FROM (SELECT *::tag, last(value) FROM go_gc_duration_seconds_count GROUP BY *) WHERE last >= 3.000) WHERE last < 4.000`),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &Transpiler{
				PromCommand: applications.PromCommand{
					Start:      tt.fields.Start,
					End:        tt.fields.End,
					Timezone:   tt.fields.Timezone,
					Evaluation: tt.fields.Evaluation,
				},
			}
			got, err := t.transpileBinaryExpr(tt.args.b)
			if (err != nil) != tt.wantErr {
				t1.Errorf("transpileBinaryExpr() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if !reflect.DeepEqual(got.String(), tt.want.String()) {
					t1.Errorf("transpileBinaryExpr() got = %v, want %v", got, tt.want)
				}
			}
		})
	}
}
