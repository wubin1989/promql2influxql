package transpiler

import (
	"github.com/influxdata/influxql"
	"github.com/prometheus/prometheus/promql/parser"
	"github.com/wubin1989/promql2influxql/command"
	"reflect"
	"testing"
	"time"
)

var endTime, endTime2, startTime2 time.Time

func TestMain(m *testing.M) {
	timezone, _ := time.LoadLocation("Asia/Shanghai")
	time.Local = timezone
	endTime = time.Date(2023, 1, 8, 10, 0, 0, 0, time.Local)
	endTime2 = time.Date(2023, 1, 6, 15, 0, 0, 0, time.Local)
	startTime2 = time.Date(2023, 1, 6, 12, 0, 0, 0, time.Local)
	m.Run()
}

func numberLiteralExpr(input string) *parser.NumberLiteral {
	expr, err := parser.ParseExpr(input)
	if err != nil {
		panic(err)
	}
	v, ok := expr.(*parser.NumberLiteral)
	if !ok {
		panic("bad input")
	}
	return v
}

func TestTranspiler_transpile(t1 *testing.T) {
	type fields struct {
		Start          *time.Time
		End            *time.Time
		Timezone       *time.Location
		Evaluation     *time.Time
		Step           time.Duration
		DataType       command.DataType
		timeRange      time.Duration
		parenExprCount int
		condition      influxql.Expr
	}
	type args struct {
		expr parser.Expr
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
				End: &endTime,
			},
			args: args{
				expr: vectorSelector(`cpu{host=~"tele.*"}`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, last(value) FROM cpu WHERE time <='2023-01-08T02:00:00Z' AND host =~ /^(?:tele.*)$/ GROUP BY *`),
			wantErr: false,
		},
		{
			name: "",
			fields: fields{
				Evaluation: &endTime,
			},
			args: args{
				expr: vectorSelector(`cpu{host=~"tele.*"}`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, last(value) FROM cpu WHERE time <='2023-01-08T02:00:00Z' AND host =~ /^(?:tele.*)$/ GROUP BY *`),
			wantErr: false,
		},
		{
			name: "",
			fields: fields{
				End: &endTime2,
			},
			args: args{
				expr: matrixSelector(`cpu{host=~"tele.*"}[5m]`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, value FROM cpu WHERE time <='2023-01-06T07:00:00Z' AND time >= '2023-01-06T06:55:00Z' AND host =~ /^(?:tele.*)$/ GROUP BY *`),
			wantErr: false,
		},
		{
			name: "",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				expr: matrixSelector(`cpu{host=~"tele.*"}[5m]`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, value FROM cpu WHERE time <= '2023-01-06T07:00:00Z' AND time >= '2023-01-06T06:55:00Z' AND host =~ /^(?:tele.*)$/ GROUP BY *`),
			wantErr: false,
		},
		{
			name: "",
			fields: fields{
				Start: &startTime2,
				End:   &endTime2,
			},
			args: args{
				expr: matrixSelector(`cpu{host=~"tele.*"}[5m]`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, value FROM cpu WHERE time <= '2023-01-06T07:00:00Z' AND time >= '2023-01-06T06:55:00Z' AND host =~ /^(?:tele.*)$/ GROUP BY *`),
			wantErr: false,
		},
		{
			name: "",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				expr: unaryExpr(`-go_gc_duration_seconds_count`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, -1 * last(value) FROM go_gc_duration_seconds_count WHERE time <= '2023-01-06T07:00:00Z' GROUP BY *`),
			wantErr: false,
		},
		{
			name: "1",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				expr: binaryExpr(`5 * go_gc_duration_seconds_count`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, 5.000 * last(value) FROM go_gc_duration_seconds_count WHERE time <= '2023-01-06T07:00:00Z' GROUP BY *`),
			wantErr: false,
		},
		{
			name: "2",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				expr: binaryExpr(`5 * 6 * go_gc_duration_seconds_count`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, 5.000 * 6.000 * last(value) FROM go_gc_duration_seconds_count WHERE time <= '2023-01-06T07:00:00Z' GROUP BY *`),
			wantErr: false,
		},
		{
			name: "3",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				expr: binaryExpr(`5 * (go_gc_duration_seconds_count - 6)`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, 5.000 * (last(value) - 6.000) FROM go_gc_duration_seconds_count WHERE time <= '2023-01-06T07:00:00Z' GROUP BY *`),
			wantErr: false,
		},
		{
			name: "4",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				expr: binaryExpr(`(5 * go_gc_duration_seconds_count) - 6`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, (5.000 * last(value)) - 6.000 FROM go_gc_duration_seconds_count WHERE time <= '2023-01-06T07:00:00Z' GROUP BY *`),
			wantErr: false,
		},
		{
			name: "5",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				expr: binaryExpr(`5 > go_gc_duration_seconds_count`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, last FROM (SELECT *::tag, last(value) FROM go_gc_duration_seconds_count GROUP BY *) WHERE time <= '2023-01-06T07:00:00Z' AND 5.000 > last`),
			wantErr: false,
		},
		{
			name: "6",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				expr: binaryExpr(`go_gc_duration_seconds_count^3`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, pow(last(value), 3.000) FROM go_gc_duration_seconds_count WHERE time <= '2023-01-06T07:00:00Z' GROUP BY *`),
			wantErr: false,
		},
		{
			name: "7",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				expr: binaryExpr(`go_gc_duration_seconds_count^3^4`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, pow(last(value), pow(3.000, 4.000)) FROM go_gc_duration_seconds_count WHERE time <= '2023-01-06T07:00:00Z' GROUP BY *`),
			wantErr: false,
		},
		{
			name: "8",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				expr: binaryExpr(`go_gc_duration_seconds_count^(3^4)`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, pow(last(value), pow(3.000, 4.000)) FROM go_gc_duration_seconds_count WHERE time <= '2023-01-06T07:00:00Z' GROUP BY *`),
			wantErr: false,
		},
		{
			name: "9",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				expr: binaryExpr(`(go_gc_duration_seconds_count^3)^4`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, pow(pow(last(value), 3.000), 4.000) FROM go_gc_duration_seconds_count WHERE time <= '2023-01-06T07:00:00Z' GROUP BY *`),
			wantErr: false,
		},
		{
			name: "10",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				expr: binaryExpr(`4^go_gc_duration_seconds_count`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, pow(4.000, last(value)) FROM go_gc_duration_seconds_count WHERE time <= '2023-01-06T07:00:00Z' GROUP BY *`),
			wantErr: false,
		},
		{
			name: "11",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				expr: binaryExpr(`go_gc_duration_seconds_count>=3<4`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, last FROM (SELECT *::tag, last FROM (SELECT *::tag, last(value) FROM go_gc_duration_seconds_count GROUP BY *) WHERE last >= 3.000) WHERE time <= '2023-01-06T07:00:00Z' AND last < 4.000`),
			wantErr: false,
		},
		{
			name: "",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				expr: binaryExpr(`sum(go_gc_duration_seconds_count>=1000) > 10000`),
			},
			want:    influxql.MustParseStatement(`SELECT sum FROM (SELECT sum(last) FROM (SELECT *::tag, last(value) FROM go_gc_duration_seconds_count GROUP BY *) WHERE last >= 1000.000) WHERE time <= '2023-01-06T07:00:00Z' AND sum > 10000.000`),
			wantErr: false,
		},
		{
			name: "",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				expr: numberLiteralExpr(`1`),
			},
			want:    influxql.MustParseExpr("1.000"),
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
					Step:       tt.fields.Step,
					DataType:   tt.fields.DataType,
				},
				timeRange:      tt.fields.timeRange,
				parenExprCount: tt.fields.parenExprCount,
				timeCondition:  tt.fields.condition,
			}
			got, err := t.transpile(tt.args.expr)
			if (err != nil) != tt.wantErr {
				t1.Errorf("transpile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got.String(), tt.want.String()) {
				t1.Errorf("transpile() got = %v, want %v", got, tt.want)
			}
		})
	}
}
