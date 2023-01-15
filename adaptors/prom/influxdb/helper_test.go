package influxdb

import (
	client "github.com/influxdata/influxdb1-client/v2"
	"github.com/influxdata/influxql"
	"github.com/prometheus/prometheus/model/timestamp"
	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/promql/parser"
	"github.com/wubin1989/promql2influxql/adaptors/prom/models"
	"reflect"
	"testing"
)

func TestQueryCommandRunner_InfluxLiteralToPromQLValue(t *testing.T) {
	type fields struct {
		Cfg     QueryCommandRunnerConfig
		Client  client.Client
		Factory *QueryCommandRunnerFactory
	}
	type args struct {
		result influxql.Literal
		cmd    models.PromCommand
	}
	tests := []struct {
		name           string
		fields         fields
		args           args
		wantValue      parser.Value
		wantResultType string
	}{
		{
			name: "",
			fields: fields{
				Cfg:     QueryCommandRunnerConfig{},
				Client:  nil,
				Factory: nil,
			},
			args: args{
				result: &influxql.IntegerLiteral{
					Val: 1,
				},
				cmd: models.PromCommand{
					End: &endTime2,
				},
			},
			wantValue: promql.Scalar{
				T: timestamp.FromTime(endTime2),
				V: float64(1),
			},
			wantResultType: string(parser.ValueTypeScalar),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receiver := &QueryCommandRunner{
				Cfg:     tt.fields.Cfg,
				Client:  tt.fields.Client,
				Factory: tt.fields.Factory,
			}
			gotValue, gotResultType := receiver.InfluxLiteralToPromQLValue(tt.args.result, tt.args.cmd)
			if !reflect.DeepEqual(gotValue, tt.wantValue) {
				t.Errorf("InfluxLiteralToPromQLValue() gotValue = %v, want %v", gotValue, tt.wantValue)
			}
			if gotResultType != tt.wantResultType {
				t.Errorf("InfluxLiteralToPromQLValue() gotResultType = %v, want %v", gotResultType, tt.wantResultType)
			}
		})
	}
}
