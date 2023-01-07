package transpiler

import (
	"github.com/influxdata/influxql"
	"github.com/prometheus/prometheus/promql/parser"
	"reflect"
	"testing"
	"time"
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
				b: binaryExpr(`5 * go_gc_duration_seconds_count`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, 5.000 * value FROM go_gc_duration_seconds_count WHERE time < '2023-01-06T07:00:00Z' GROUP BY * LIMIT 1`),
			wantErr: false,
		},
		//{
		//	name: "",
		//	fields: fields{
		//		Evaluation: &endTime2,
		//	},
		//	args: args{
		//		b: binaryExpr(`5 * rate(go_gc_duration_seconds_count[1m])`),
		//	},
		//	want:    influxql.MustParseStatement(`SELECT *::tag, 5.000 * value FROM go_gc_duration_seconds_count WHERE time < '2023-01-06T07:00:00Z' GROUP BY * LIMIT 1`),
		//	wantErr: false,
		//},
		{
			name: "2",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				b: binaryExpr(`5 * 6 * go_gc_duration_seconds_count`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, 5.000 * 6.000 * value FROM go_gc_duration_seconds_count WHERE time < '2023-01-06T07:00:00Z' GROUP BY * LIMIT 1`),
			wantErr: false,
		},
		{
			name: "3",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				b: binaryExpr(`5 * (go_gc_duration_seconds_count - 6)`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, 5.000 * (value - 6.000) FROM go_gc_duration_seconds_count WHERE time < '2023-01-06T07:00:00Z' GROUP BY * LIMIT 1`),
			wantErr: false,
		},
		{
			name: "4",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				b: binaryExpr(`(5 * go_gc_duration_seconds_count) - 6`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, (5.000 * value) - 6.000 FROM go_gc_duration_seconds_count WHERE time < '2023-01-06T07:00:00Z' GROUP BY * LIMIT 1`),
			wantErr: false,
		},
		{
			name: "5",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				b: binaryExpr(`5 > go_gc_duration_seconds_count`),
			},
			want:    influxql.MustParseStatement(`SELECT * FROM go_gc_duration_seconds_count WHERE time < '2023-01-06T07:00:00Z' AND 5.000 > value GROUP BY * LIMIT 1`),
			wantErr: false,
		},
		{
			name: "6",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				b: binaryExpr(`go_gc_duration_seconds_count^3`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, pow(value, 3.000) FROM go_gc_duration_seconds_count WHERE time < '2023-01-06T07:00:00Z' GROUP BY * LIMIT 1`),
			wantErr: false,
		},
		{
			name: "7",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				b: binaryExpr(`go_gc_duration_seconds_count^3^4`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, pow(value, pow(3.000, 4.000)) FROM go_gc_duration_seconds_count WHERE time < '2023-01-06T07:00:00Z' GROUP BY * LIMIT 1`),
			wantErr: false,
		},
		{
			name: "8",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				b: binaryExpr(`go_gc_duration_seconds_count^(3^4)`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, pow(value, pow(3.000, 4.000)) FROM go_gc_duration_seconds_count WHERE time < '2023-01-06T07:00:00Z' GROUP BY * LIMIT 1`),
			wantErr: false,
		},
		{
			name: "9",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				b: binaryExpr(`(go_gc_duration_seconds_count^3)^4`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, pow(pow(value, 3.000), 4.000) FROM go_gc_duration_seconds_count WHERE time < '2023-01-06T07:00:00Z' GROUP BY * LIMIT 1`),
			wantErr: false,
		},
		{
			name: "10",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				b: binaryExpr(`4^go_gc_duration_seconds_count`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, pow(4.000, value) FROM go_gc_duration_seconds_count WHERE time < '2023-01-06T07:00:00Z' GROUP BY * LIMIT 1`),
			wantErr: false,
		},
		{
			name: "11",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				b: binaryExpr(`go_gc_duration_seconds_count>=3<4`),
			},
			want:    influxql.MustParseStatement(`SELECT * FROM go_gc_duration_seconds_count WHERE time < '2023-01-06T07:00:00Z' AND value >= 3.000 AND value < 4.000 GROUP BY * LIMIT 1`),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &Transpiler{
				Start:      tt.fields.Start,
				End:        tt.fields.End,
				Timezone:   tt.fields.Timezone,
				Evaluation: tt.fields.Evaluation,
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