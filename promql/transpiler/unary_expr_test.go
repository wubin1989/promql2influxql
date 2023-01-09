package transpiler

import (
	"github.com/influxdata/influxql"
	"github.com/prometheus/prometheus/promql/parser"
	"reflect"
	"testing"
	"time"
)

func unaryExpr(input string) *parser.UnaryExpr {
	expr, err := parser.ParseExpr(input)
	if err != nil {
		panic(err)
	}
	v, ok := expr.(*parser.UnaryExpr)
	if !ok {
		panic("bad input")
	}
	return v
}

func TestTranspiler_transpileUnaryExpr(t1 *testing.T) {
	type fields struct {
		Start      *time.Time
		End        *time.Time
		Timezone   *time.Location
		Evaluation *time.Time
	}
	type args struct {
		ue *parser.UnaryExpr
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
				ue: unaryExpr(`-(3 * go_gc_duration_seconds_count)`),
			},
			want:    influxql.MustParseStatement(`SELECT *::tag, -1 * (3.000 * last(value)) FROM go_gc_duration_seconds_count GROUP BY *`),
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
			got, err := t.transpileUnaryExpr(tt.args.ue)
			if (err != nil) != tt.wantErr {
				t1.Errorf("transpileUnaryExpr() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got.String(), tt.want.String()) {
				t1.Errorf("transpileUnaryExpr() got = %v, want %v", got, tt.want)
			}
		})
	}
}
