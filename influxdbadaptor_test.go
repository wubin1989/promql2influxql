package promql2influxql

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/golang/mock/gomock"
	client "github.com/influxdata/influxdb1-client/v2"
	"github.com/unionj-cloud/go-doudou/v2/toolkit/copier"
	"github.com/wubin1989/promql2influxql/command"
	"github.com/wubin1989/promql2influxql/config"
	"github.com/wubin1989/promql2influxql/influxql/mock"
	"github.com/wubin1989/promql2influxql/promql"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

var timezone *time.Location
var testDir = "testdata"
var endTime, endTime2, startTime2 time.Time

func TestMain(m *testing.M) {
	timezone, _ = time.LoadLocation("Asia/Shanghai")
	time.Local = timezone

	endTime = time.Date(2023, 1, 8, 10, 0, 0, 0, time.Local)
	endTime2 = time.Date(2023, 1, 6, 15, 0, 0, 0, time.Local)
	startTime2 = time.Date(2023, 1, 6, 12, 0, 0, 0, time.Local)

	m.Run()
}

func MustParseDuration(s string, t *testing.T) time.Duration {
	result, err := time.ParseDuration(s)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestInfluxDBAdaptor_Query(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	database := "telegraf"
	influxCmd := "SELECT *::tag, last(usage_idle) FROM cpu WHERE time <= '2023-01-06T07:00:00Z' AND host =~ /^(?:tele.*)$/ GROUP BY * TZ('Asia/Shanghai')"

	var response client.Response
	testFile := filepath.Join(testDir, "querycommandrunner_test_response.json")
	responseJson, err := ioutil.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(responseJson, &response); err != nil {
		t.Fatal(err)
	}

	mockClient := mock.NewMockClient(ctrl)
	mockClient.
		EXPECT().Query(client.NewQuery(influxCmd, database, "")).
		Return(&response, nil).
		AnyTimes()

	expectedJson := `{"Result":[{"metric":{"__name__":"cpu","cpu":"cpu-total","host":"telegraf"},"value":[1673336340,"86.09237156206318"]},{"metric":{"__name__":"cpu","cpu":"cpu0","host":"telegraf"},"value":[1673336340,"84.93292053672343"]},{"metric":{"__name__":"cpu","cpu":"cpu1","host":"telegraf"},"value":[1673336340,"87.17413972880925"]}],"ResultType":"vector","Error":null}`

	var expected map[string]interface{}
	if err = json.Unmarshal([]byte(expectedJson), &expected); err != nil {
		t.Fatal(err)
	}

	type fields struct {
		_      [0]int
		Cfg    config.Config
		Client client.Client
	}
	type args struct {
		ctx context.Context
		cmd command.Command
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			name: "",
			fields: fields{
				Cfg: config.Config{
					Timeout: MustParseDuration("1m", t),
					Verbose: true,
				},
				Client: mockClient,
			},
			args: args{
				ctx: context.Background(),
				cmd: command.Command{
					Cmd:           `cpu{host=~"tele.*"}`,
					Dialect:       promql.PROMQL_DIALECT,
					Database:      "telegraf",
					Start:         nil,
					End:           &endTime2,
					Timezone:      timezone,
					Evaluation:    nil,
					Step:          0,
					DataType:      0,
					ValueFieldKey: "usage_idle",
				},
			},
			want:    expected,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receiver := &InfluxDBAdaptor{
				Cfg:    tt.fields.Cfg,
				Client: tt.fields.Client,
			}
			got, err := receiver.Query(tt.args.ctx, tt.args.cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("Query() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			gotJ, _ := json.Marshal(got)
			fmt.Println(string(gotJ))
			var gotCopy map[string]interface{}
			copier.DeepCopy(got, &gotCopy)
			if !reflect.DeepEqual(gotCopy, tt.want) {
				t.Errorf("Run() got = %v, want %v", got, tt.want)
			}
		})
	}
}
