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

var endTime, endTime2, startTime2 time.Time
var timezone *time.Location

func TestMain(m *testing.M) {
	timezone, _ = time.LoadLocation("Asia/Shanghai")
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
		Database       string
		LabelName      string
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
				expr: testinghelper.VectorSelector(`cpu{host=~"tele.*"}`),
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
				expr: testinghelper.VectorSelector(`cpu{host=~"tele.*"}`),
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
				expr: testinghelper.MatrixSelector(`cpu{host=~"tele.*"}[5m]`),
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
				expr: testinghelper.MatrixSelector(`cpu{host=~"tele.*"}[5m]`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, value FROM cpu WHERE time <= '2023-01-06T07:00:00Z' AND time >= '2023-01-06T06:55:00Z' AND host =~ /^(?:tele.*)$/ GROUP BY *`),
			wantErr: false,
		},
		{
			name: `invalid expression type "range vector" for range query, must be Scalar or instant Vector`,
			fields: fields{
				Start: &startTime2,
				End:   &endTime2,
			},
			args: args{
				expr: testinghelper.MatrixSelector(`cpu{host=~"tele.*"}[5m]`),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				expr: testinghelper.UnaryExpr(`-go_gc_duration_seconds_count`),
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
				expr: testinghelper.BinaryExpr(`5 * go_gc_duration_seconds_count`),
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
				expr: testinghelper.BinaryExpr(`5 * 6 * go_gc_duration_seconds_count`),
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
				expr: testinghelper.BinaryExpr(`5 * (go_gc_duration_seconds_count - 6)`),
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
				expr: testinghelper.BinaryExpr(`(5 * go_gc_duration_seconds_count) - 6`),
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
				expr: testinghelper.BinaryExpr(`5 > go_gc_duration_seconds_count`),
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
				expr: testinghelper.BinaryExpr(`go_gc_duration_seconds_count^3`),
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
				expr: testinghelper.BinaryExpr(`go_gc_duration_seconds_count^3^4`),
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
				expr: testinghelper.BinaryExpr(`go_gc_duration_seconds_count^(3^4)`),
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
				expr: testinghelper.BinaryExpr(`(go_gc_duration_seconds_count^3)^4`),
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
				expr: testinghelper.BinaryExpr(`4^go_gc_duration_seconds_count`),
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
				expr: testinghelper.BinaryExpr(`go_gc_duration_seconds_count>=3<4`),
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
				expr: testinghelper.BinaryExpr(`sum(go_gc_duration_seconds_count>=1000) > 10000`),
			},
			want:    influxql.MustParseStatement(`SELECT sum FROM (SELECT sum(last) FROM (SELECT *::tag, last(value) FROM go_gc_duration_seconds_count GROUP BY *) WHERE last >= 1000.000) WHERE time <= '2023-01-06T07:00:00Z' AND sum > 10000.000`),
			wantErr: false,
		},
		{
			name: "not support both sides has VectorSelector in binary expression",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				expr: testinghelper.BinaryExpr("avg(node_load5{instance=\"\",job=\"\"}) /  count(count(node_cpu_seconds_total{instance=\"\",job=\"\"}) by (cpu)) * 100"),
			},
			want:    nil,
			wantErr: true,
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
		{
			name: "",
			fields: fields{
				Evaluation: &endTime2,
				Start:      &startTime2,
				DataType:   command.GRAPH_DATA,
			},
			args: args{
				expr: testinghelper.CallExpr(`sum_over_time(go_gc_duration_seconds_count[5m])`),
			},
			want:    influxql.MustParseStatement(`SELECT sum(value) FROM go_gc_duration_seconds_count WHERE time <= '2023-01-06T07:00:00Z' AND time >= '2023-01-06T04:00:00Z' GROUP BY *, time(5m)`),
			wantErr: false,
		},
		{
			name: "not support PromQL subquery expression",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				expr: testinghelper.SubqueryExpr(`sum_over_time(go_gc_duration_seconds_count[5m])[1h:10m]`),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				expr: testinghelper.StringLiteralExpr(`"justastring"`),
			},
			want:    influxql.MustParseExpr("'justastring'"),
			wantErr: false,
		},
		{
			name: "",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				expr: testinghelper.BinaryExpr(`-10 * cpu`),
			},
			want:    influxql.MustParseStatement("SELECT *::tag, -10.000 * last(value) FROM cpu WHERE time <= '2023-01-06T07:00:00Z' GROUP BY *"),
			wantErr: false,
		},
		{
			name: "",
			fields: fields{
				Start:    &startTime2,
				End:      &endTime2,
				Timezone: timezone,
			},
			args: args{
				expr: testinghelper.VectorSelector(`cpu{host="telegraf"}`),
			},
			want:    influxql.MustParseStatement("SELECT *::tag, value FROM cpu WHERE time <= '2023-01-06T07:00:00Z' AND time >= '2023-01-06T04:00:00Z' AND host = 'telegraf' GROUP BY * TZ('Asia/Shanghai')"),
			wantErr: false,
		},
		{
			name: "",
			fields: fields{
				Evaluation: &endTime,
				DataType:   command.LABEL_VALUES_DATA,
				Database:   "prometheus",
				LabelName:  "job",
			},
			args: args{
				expr: testinghelper.VectorSelector(`go_goroutines{instance=~"192.168.*"}`),
			},
			want:    influxql.MustParseStatement(`SHOW TAG VALUES ON prometheus FROM go_goroutines WITH KEY = job WHERE time <= '2023-01-08T02:00:00Z' AND instance =~ /^(?:192.168.*)$/`),
			wantErr: false,
		},
		{
			name: "",
			fields: fields{
				Evaluation: &endTime,
				DataType:   command.LABEL_VALUES_DATA,
				Database:   "prometheus",
				LabelName:  "job",
			},
			args: args{
				expr: testinghelper.VectorSelector(`go_goroutines{instance=~"192.168.*"}`),
			},
			want:    influxql.MustParseStatement(`SHOW TAG VALUES ON prometheus FROM go_goroutines WITH KEY = job WHERE time <= '2023-01-08T02:00:00Z' AND instance =~ /^(?:192.168.*)$/`),
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
					Database:   tt.fields.Database,
					LabelName:  tt.fields.LabelName,
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
			if got != nil {
				if !reflect.DeepEqual(got.String(), tt.want.String()) {
					t1.Errorf("transpile() got = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestCondition_Or(t *testing.T) {
	type args struct {
		expr *influxql.BinaryExpr
	}
	tests := []struct {
		name     string
		receiver *Condition
		args     args
		want     *Condition
	}{
		{
			name: "",
			receiver: &Condition{
				Op: influxql.EQREGEX,
				LHS: &influxql.VarRef{
					Val: "cpu",
				},
				RHS: &influxql.StringLiteral{
					Val: "cpu.*",
				},
			},
			args: args{
				expr: &influxql.BinaryExpr{
					Op: influxql.EQ,
					LHS: &influxql.VarRef{
						Val: "host",
					},
					RHS: &influxql.StringLiteral{
						Val: "prometheus-server",
					},
				},
			},
			want: &Condition{
				Op: influxql.OR,
				LHS: &influxql.BinaryExpr{
					Op: influxql.EQREGEX,
					LHS: &influxql.VarRef{
						Val: "cpu",
					},
					RHS: &influxql.StringLiteral{
						Val: "cpu.*",
					},
				},
				RHS: &influxql.BinaryExpr{
					Op: influxql.EQ,
					LHS: &influxql.VarRef{
						Val: "host",
					},
					RHS: &influxql.StringLiteral{
						Val: "prometheus-server",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.receiver.Or(tt.args.expr); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Or() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_makeInt64Pointer(t *testing.T) {
	var a int64 = 1
	type args struct {
		val int64
	}
	tests := []struct {
		name string
		args args
		want *int64
	}{
		{
			name: "",
			args: args{
				val: 1,
			},
			want: &a,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := makeInt64Pointer(tt.args.val); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("makeInt64Pointer() = %v, want %v", got, tt.want)
			}
		})
	}
}
