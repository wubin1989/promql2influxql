package service

import (
	"context"
	"encoding/json"
	"fmt"
	client "github.com/influxdata/influxdb1-client/v2"
	"github.com/unionj-cloud/go-doudou/v2/toolkit/copier"
	"github.com/wubin1989/promql2influxql"
	adaptorCfg "github.com/wubin1989/promql2influxql/config"
	"github.com/wubin1989/promql2influxql/rpc/config"
	"reflect"
	"testing"
	"time"
)

var adaptor *promql2influxql.InfluxDBAdaptor
var conf *config.Config
var endTime time.Time

func TestMain(m *testing.M) {
	timezone, _ := time.LoadLocation("Asia/Shanghai")
	time.Local = timezone
	conf = config.LoadFromEnv()
	endTime = time.Date(2023, 1, 6, 15, 0, 0, 0, time.Local)

	var (
		err          error
		influxClient client.Client
	)
	influxClient, err = client.NewHTTPClient(client.HTTPConfig{
		Addr:      conf.BizConf.AdaptorInfluxAddr,
		Username:  conf.BizConf.AdaptorInfluxUsername,
		Password:  conf.BizConf.AdaptorInfluxPassword,
		UserAgent: "promql2influxql",
		Timeout:   conf.BizConf.AdaptorInfluxClientTimeout,
	})
	if err != nil {
		panic(err)
	}
	defer influxClient.Close()

	adaptor = promql2influxql.NewInfluxDBAdaptor(adaptorCfg.Config{
		Timeout: conf.BizConf.AdaptorTimeout,
		Verbose: conf.BizConf.AdaptorVerbose,
	}, influxClient)

	m.Run()
}

func TestRpcImpl_Query(t *testing.T) {
	evaluationTs := fmt.Sprintf("%.2f", float64(endTime.UnixMilli())/1000)

	expectedJson := `{"result":[1672988400,"2"],"resultType":"scalar"}`
	var expected map[string]interface{}
	if err := json.Unmarshal([]byte(expectedJson), &expected); err != nil {
		t.Fatal(err)
	}

	type fields struct {
		conf    *config.Config
		adaptor *promql2influxql.InfluxDBAdaptor
	}
	type args struct {
		ctx     context.Context
		query   string
		t       *string
		timeout *string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantRet map[string]interface{}
		wantErr bool
	}{
		{
			name: "",
			fields: fields{
				conf:    conf,
				adaptor: adaptor,
			},
			args: args{
				ctx:   context.Background(),
				query: "1+1",
				t:     &evaluationTs,
			},
			wantRet: expected,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receiver := &RpcImpl{
				conf:    tt.fields.conf,
				adaptor: tt.fields.adaptor,
			}
			gotRet, _, err := receiver.Query(tt.args.ctx, tt.args.query, tt.args.t, tt.args.timeout)
			if (err != nil) != tt.wantErr {
				t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			gotJ, _ := json.Marshal(gotRet)
			fmt.Println(string(gotJ))
			var gotCopy map[string]interface{}
			copier.DeepCopy(gotRet, &gotCopy)
			if !reflect.DeepEqual(gotCopy, tt.wantRet) {
				t.Errorf("Run() got = %v, want %v", gotCopy, tt.wantRet)
			}
		})
	}
}
