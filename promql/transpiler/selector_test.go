package transpiler

import (
	"github.com/influxdata/influxql"
	"github.com/prometheus/prometheus/promql/parser"
	"github.com/wubin1989/promql2influxql/command"
	"reflect"
	"testing"
	"time"
)

func vectorSelector(input string) *parser.VectorSelector {
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

func matrixSelector(input string) *parser.MatrixSelector {
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

func TestTranspiler_TranspileVectorSelector2ConditionExpr(t1 *testing.T) {
	type fields struct {
		Start *time.Time
		End   *time.Time
	}
	type args struct {
		v *parser.VectorSelector
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    influxql.Expr
		wantErr bool
	}{
		{
			name: "",
			fields: fields{
				Start: nil,
				End:   &endTime,
			},
			args: args{
				v: vectorSelector(`cpu{host=~"tele.*"}`),
			},
			want:    influxql.MustParseExpr("host =~ /^(?:tele.*)$/"),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &Transpiler{
				Command: command.Command{
					Start: tt.fields.Start,
					End:   tt.fields.End,
				},
			}
			_, got, err := t.transpileVectorSelector2ConditionExpr(tt.args.v)
			if (err != nil) != tt.wantErr {
				t1.Errorf("transpileVectorSelector2ConditionExpr() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got.String(), tt.want.String()) {
				t1.Errorf("transpileVectorSelector2ConditionExpr() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTranspiler_transpileInstantVectorSelector(t1 *testing.T) {
	type fields struct {
		Start      *time.Time
		End        *time.Time
		Timezone   *time.Location
		Evaluation *time.Time
	}
	type args struct {
		v *parser.VectorSelector
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
				v: vectorSelector(`cpu{host=~"tele.*"}`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, last(value) FROM cpu WHERE host =~ /^(?:tele.*)$/ GROUP BY *`),
			wantErr: false,
		},
		{
			name: "",
			fields: fields{
				Evaluation: &endTime,
			},
			args: args{
				v: vectorSelector(`cpu{host=~"tele.*"}`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, last(value) FROM cpu WHERE host =~ /^(?:tele.*)$/ GROUP BY *`),
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
			}
			got, err := t.transpileInstantVectorSelector(tt.args.v)
			if (err != nil) != tt.wantErr {
				t1.Errorf("transpileInstantVectorSelector() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got.String(), tt.want.String()) {
				t1.Errorf("transpileInstantVectorSelector() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTranspiler_transpileRangeVectorSelector(t1 *testing.T) {
	type fields struct {
		Start      *time.Time
		End        *time.Time
		Timezone   *time.Location
		Evaluation *time.Time
	}
	type args struct {
		v *parser.MatrixSelector
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
				End: &endTime2,
			},
			args: args{
				v: matrixSelector(`cpu{host=~"tele.*"}[5m]`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, value FROM cpu WHERE host =~ /^(?:tele.*)$/ GROUP BY *`),
			wantErr: false,
		},
		{
			name: "",
			fields: fields{
				Evaluation: &endTime2,
			},
			args: args{
				v: matrixSelector(`cpu{host=~"tele.*"}[5m]`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, value FROM cpu WHERE host =~ /^(?:tele.*)$/ GROUP BY *`),
			wantErr: false,
		},
		{
			name: "",
			fields: fields{
				Start: &startTime2,
				End:   &endTime2,
			},
			args: args{
				v: matrixSelector(`cpu{host=~"tele.*"}[5m]`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, value FROM cpu WHERE host =~ /^(?:tele.*)$/ GROUP BY *`),
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
			}
			got, err := t.transpileRangeVectorSelector(tt.args.v)
			if (err != nil) != tt.wantErr {
				t1.Errorf("transpileRangeVectorSelector() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got.String(), tt.want.String()) {
				t1.Errorf("transpileRangeVectorSelector() got = %v, want %v", got, tt.want)
			}
		})
	}
}
