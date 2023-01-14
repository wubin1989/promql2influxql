package influxdb

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/golang/mock/gomock"
	_ "github.com/influxdata/influxdb1-client"
	client "github.com/influxdata/influxdb1-client/v2"
	"github.com/stretchr/testify/require"
	"github.com/unionj-cloud/go-doudou/v2/toolkit/copier"
	"github.com/wubin1989/promql2influxql/applications"
	"github.com/wubin1989/promql2influxql/influxql/mock"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

var influxClient client.Client
var timezone *time.Location
var testDir = "testdata"
var endTime, endTime2, startTime2, endTime3 time.Time

func TestMain(m *testing.M) {
	var err error
	influxClient, err = client.NewHTTPClient(client.HTTPConfig{
		Addr: "http://192.168.98.151:8086",
	})
	if err != nil {
		panic(err)
	}
	defer influxClient.Close()

	timezone, _ = time.LoadLocation("Asia/Shanghai")
	time.Local = timezone

	endTime = time.Date(2023, 1, 8, 10, 0, 0, 0, time.Local)
	endTime2 = time.Date(2023, 1, 6, 15, 0, 0, 0, time.Local)
	startTime2 = time.Date(2023, 1, 6, 12, 0, 0, 0, time.Local)
	endTime3 = time.Date(2023, 1, 15, 12, 0, 0, 0, time.Local)

	m.Run()
}

func MustParseDuration(s string, t *testing.T) time.Duration {
	result, err := time.ParseDuration(s)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestQueryCommandRunner_Run_Vector_Table(t *testing.T) {
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
		Cfg     AdaptorConfig
		Client  client.Client
		Factory *QueryCommandRunnerFactory
	}
	type args struct {
		ctx context.Context
		cmd applications.PromCommand
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
				Cfg: AdaptorConfig{
					Timeout: MustParseDuration("1m", t),
					Verbose: true,
				},
				Client:  mockClient,
				Factory: queryCommandRunnerFactory,
			},
			args: args{
				ctx: context.Background(),
				cmd: applications.PromCommand{
					Cmd:           `cpu{host=~"tele.*"}`,
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
			receiver := &QueryCommandRunner{
				Cfg:     tt.fields.Cfg,
				Client:  tt.fields.Client,
				Factory: tt.fields.Factory,
			}
			got, err := receiver.Run(tt.args.ctx, tt.args.cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			got = got.(RunResult)
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

func TestQueryCommandRunner_Run_Vector_Table1(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	database := "telegraf"
	influxCmd := "SELECT *::tag, top(max, 3) FROM (SELECT *::tag, max(usage_idle) FROM cpu WHERE host =~ /^(?:tele.*)$/ GROUP BY *) WHERE time <= '2023-01-06T07:00:00Z' AND time >= '2023-01-06T06:55:00Z' TZ('Asia/Shanghai')"

	var response client.Response
	testFile := filepath.Join(testDir, "querycommandrunner_test_response1.json")
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

	expectedJson := `{"Result":[{"metric":{"__name__":"cpu","cpu":"cpu1","host":"telegraf"},"value":[1672988200,"90.67357512958837"]},{"metric":{"__name__":"cpu","cpu":"cpu-total","host":"telegraf"},"value":[1672988200,"90.08307372792021"]},{"metric":{"__name__":"cpu","cpu":"cpu0","host":"telegraf"},"value":[1672988200,"89.38605619117084"]}],"ResultType":"vector","Error":null}`

	var expected map[string]interface{}
	if err = json.Unmarshal([]byte(expectedJson), &expected); err != nil {
		t.Fatal(err)
	}

	type fields struct {
		Cfg     AdaptorConfig
		Client  client.Client
		Factory *QueryCommandRunnerFactory
	}
	type args struct {
		ctx context.Context
		cmd applications.PromCommand
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
				Cfg: AdaptorConfig{
					Timeout: MustParseDuration("1m", t),
					Verbose: true,
				},
				Client:  mockClient,
				Factory: queryCommandRunnerFactory,
			},
			args: args{
				ctx: context.Background(),
				cmd: applications.PromCommand{
					Cmd:           `topk(3, max_over_time(cpu{host=~"tele.*"}[5m]))`,
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
			receiver := &QueryCommandRunner{
				Cfg:     tt.fields.Cfg,
				Client:  tt.fields.Client,
				Factory: tt.fields.Factory,
			}
			got, err := receiver.Run(tt.args.ctx, tt.args.cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			got = got.(RunResult)
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

func TestQueryCommandRunner_Run_Matrix_Table(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	database := "telegraf"
	influxCmd := "SELECT *::tag, usage_idle FROM cpu WHERE time <= '2023-01-06T07:00:00Z' AND time >= '2023-01-06T06:55:00Z' AND host =~ /^(?:tele.*)$/ GROUP BY * TZ('Asia/Shanghai')"

	var response client.Response
	testFile := filepath.Join(testDir, "querycommandrunner_test_response2.json")
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

	expectedJson := `{"Result":[{"metric":{"__name__":"cpu","cpu":"cpu-total","host":"telegraf"},"values":[[1672988100,"72.4102030194194"],[1672988110,"85.52012545730291"],[1672988120,"84.715025906817"],[1672988130,"74.83134405811555"],[1672988140,"86.54147104856867"],[1672988150,"80.04168837928174"],[1672988160,"73.41311134231485"],[1672988170,"86.69438669439235"],[1672988180,"83.16883116887955"],[1672988190,"71.7842323652398"],[1672988200,"90.08307372792021"],[1672988210,"80.94251683054084"],[1672988220,"72.06878582602721"],[1672988230,"87.59162303654391"],[1672988240,"86.73155737693064"],[1672988250,"72.66149870808573"],[1672988260,"86.89119170984269"],[1672988270,"82.13914849428522"],[1672988280,"75.76703068133696"],[1672988290,"87.5064800413274"],[1672988300,"85.50724637698407"],[1672988310,"71.69614984379362"],[1672988320,"86.42487046633718"],[1672988330,"81.38325533026178"],[1672988340,"69.47314049576916"],[1672988350,"89.24508790092246"],[1672988360,"87.23514211872967"],[1672988370,"71.35497166406488"],[1672988380,"87.4739039667626"],[1672988390,"81.52965660763854"],[1672988400,"72.8971962615382"]]},{"metric":{"__name__":"cpu","cpu":"cpu0","host":"telegraf"},"values":[[1672988100,"70.89783281727237"],[1672988110,"85.22372528605027"],[1672988120,"83.74741200846746"],[1672988130,"73.85892116182134"],[1672988140,"84.71074380168395"],[1672988150,"78.28335056858747"],[1672988160,"72.02072538876511"],[1672988170,"85.49222797932617"],[1672988180,"82.71221532078467"],[1672988190,"69.98972250781817"],[1672988200,"89.38605619117084"],[1672988210,"79.40267765212596"],[1672988220,"70.8333333333207"],[1672988230,"86.75703858196053"],[1672988240,"85.49695740344904"],[1672988250,"71.31147540983362"],[1672988260,"86.18556701047044"],[1672988270,"81.78053830218761"],[1672988280,"74.38271604933189"],[1672988290,"87.04663212423279"],[1672988300,"84.43298969074763"],[1672988310,"70.35123966959853"],[1672988320,"85.34571723413349"],[1672988330,"79.81462409893413"],[1672988340,"67.96714579072099"],[1672988350,"88.39835728935408"],[1672988360,"87.02368692074337"],[1672988370,"70.57613168724033"],[1672988380,"87.00623700623072"],[1672988390,"80.39215686280988"],[1672988400,"71.00103199162488"]]},{"metric":{"__name__":"cpu","cpu":"cpu1","host":"telegraf"},"values":[[1672988100,"73.76705141657602"],[1672988110,"85.81932773116854"],[1672988120,"85.77362409143633"],[1672988130,"75.80477673919863"],[1672988140,"88.60759493697506"],[1672988150,"81.74186778588194"],[1672988160,"74.81713688608428"],[1672988170,"87.89144050087609"],[1672988180,"83.64583333352408"],[1672988190,"73.68972746323722"],[1672988200,"90.67357512958837"],[1672988210,"82.43243243230161"],[1672988220,"73.43096234322807"],[1672988230,"88.25995807106841"],[1672988240,"88.07053941912879"],[1672988250,"74.1397288842875"],[1672988260,"87.40894901139961"],[1672988270,"82.60416666679424"],[1672988280,"77.1819137748614"],[1672988290,"87.87564766855536"],[1672988300,"86.75703858180879"],[1672988310,"73.01255230123039"],[1672988320,"87.40894901139961"],[1672988330,"83.07045215573733"],[1672988340,"70.8808290155506"],[1672988350,"90.19812304468853"],[1672988360,"87.34439834036797"],[1672988370,"72.23942208460596"],[1672988380,"88.22292323860358"],[1672988390,"82.49475890973397"],[1672988400,"74.81713688631181"]]}],"ResultType":"matrix","Error":null}`

	var expected map[string]interface{}
	if err = json.Unmarshal([]byte(expectedJson), &expected); err != nil {
		t.Fatal(err)
	}

	type fields struct {
		Cfg     AdaptorConfig
		Client  client.Client
		Factory *QueryCommandRunnerFactory
	}
	type args struct {
		ctx context.Context
		cmd applications.PromCommand
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
				Cfg: AdaptorConfig{
					Timeout: MustParseDuration("1m", t),
					Verbose: true,
				},
				Client:  mockClient,
				Factory: queryCommandRunnerFactory,
			},
			args: args{
				ctx: context.Background(),
				cmd: applications.PromCommand{
					Cmd:           `cpu{host=~"tele.*"}[5m]`,
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
			receiver := &QueryCommandRunner{
				Cfg:     tt.fields.Cfg,
				Client:  tt.fields.Client,
				Factory: tt.fields.Factory,
			}
			got, err := receiver.Run(tt.args.ctx, tt.args.cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			got = got.(RunResult)
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

func TestQueryCommandRunner_Run_Matrix_Graph(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	database := "telegraf"
	influxCmd := "SELECT *::tag, max(usage_idle) FROM cpu WHERE time <= '2023-01-06T07:00:00Z' AND time >= '2023-01-06T04:00:00Z' AND host =~ /^(?:tele.*)$/ GROUP BY *, time(5m) TZ('Asia/Shanghai')"

	var response client.Response
	testFile := filepath.Join(testDir, "querycommandrunner_test_response3.json")
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

	expectedJson := `{"Result":[{"metric":{"__name__":"cpu","cpu":"cpu-total","host":"telegraf"},"values":[[1672977600,"90.74457083759839"],[1672977900,"90.75324675339002"],[1672978200,"90.31413612562693"],[1672978500,"90.60887512896518"],[1672978800,"89.13721413722703"],[1672979100,"88.36006207967873"],[1672979400,"91.61021365301805"],[1672979700,"90.26411185911505"],[1672980000,"90.93277748832477"],[1672980300,"90.05208333332118"],[1672980600,"90.03147953819195"],[1672980900,"89.74895397493552"],[1672981200,"89.86415882965589"],[1672981500,"90.03131524020385"],[1672981800,"90.83810515352737"],[1672982100,"89.53427524855331"],[1672982400,"90.89490114462777"],[1672982700,"90.45643153542379"],[1672983000,"89.66421825812485"],[1672983300,"91.16735537208687"],[1672983600,"90.10256410271963"],[1672983900,"91.60621761659938"],[1672984200,"90.92331768379768"],[1672984500,"89.690721649616"],[1672984800,"89.66770508816808"],[1672985100,"91.13070539409503"],[1672985400,"90.49095607223623"],[1672985700,"91.95402298846905"],[1672986000,"90.30745179773368"],[1672986300,"91.20082815729282"],[1672986600,"90.0207900208699"],[1672986900,"91.16883116864621"],[1672987200,"90.68450849203128"],[1672987500,"90.39958484689137"],[1672987800,"90.79627714595279"],[1672988100,"90.08307372792021"],[1672988400,"72.8971962615382"]]},{"metric":{"__name__":"cpu","cpu":"cpu0","host":"telegraf"},"values":[[1672977600,"90.20618556703738"],[1672977900,"89.62655601659688"],[1672978200,"89.35950413225174"],[1672978500,"89.73305954826014"],[1672978800,"86.57024793386474"],[1672979100,"90.6952965234944"],[1672979400,"92.32343909928672"],[1672979700,"88.70466321246461"],[1672980000,"89.9377593360869"],[1672980300,"88.96982310094472"],[1672980600,"89.51781970650791"],[1672980900,"88.96982310096142"],[1672981200,"89.31140801637174"],[1672981500,"88.94681960377416"],[1672981800,"90.95634095638498"],[1672982100,"88.72651357005597"],[1672982400,"91.53766769873853"],[1672982700,"90.3292181071228"],[1672983000,"89.20041536871138"],[1672983300,"90.81527347794298"],[1672983600,"89.73577235777317"],[1672983900,"90.67357512931491"],[1672984200,"89.93775933622267"],[1672984500,"88.79753340201395"],[1672984800,"88.72802481919553"],[1672985100,"90.75804776752726"],[1672985400,"89.51695786236921"],[1672985700,"93.82716049376802"],[1672986000,"89.61578400831239"],[1672986300,"89.18640576709376"],[1672986600,"88.75128998956865"],[1672986900,"90.16563147001298"],[1672987200,"90.36885245900173"],[1672987500,"93.32648870641889"],[1672987800,"90.08264462802588"],[1672988100,"89.38605619117084"],[1672988400,"71.00103199162488"]]},{"metric":{"__name__":"cpu","cpu":"cpu1","host":"telegraf"},"values":[[1672977600,"91.4315569487316"],[1672977900,"91.97916666661693"],[1672978200,"91.58780231347214"],[1672978500,"91.5800415800617"],[1672978800,"93.0062630480362"],[1672979100,"88.67924528301023"],[1672979400,"92.73858921150801"],[1672979700,"91.99584199580788"],[1672980000,"91.64926931089664"],[1672980300,"91.04166666665246"],[1672980600,"90.71729957814522"],[1672980900,"90.63157894737455"],[1672981200,"90.90909090913237"],[1672981500,"91.29979035636998"],[1672981800,"91.39559286471633"],[1672982100,"90.24134312690178"],[1672982400,"91.16424116418975"],[1672982700,"90.59561128518209"],[1672983000,"90.04237288133372"],[1672983300,"92.04188481679908"],[1672983600,"90.38262668023901"],[1672983900,"92.53886010361038"],[1672984200,"92.11356466877744"],[1672984500,"90.76763485464025"],[1672984800,"90.53069719025379"],[1672985100,"91.50259067364841"],[1672985400,"91.476091476166"],[1672985700,"89.81972428427824"],[1672986000,"91.18572927600619"],[1672986300,"93.34027055165664"],[1672986600,"91.39559286471633"],[1672986900,"91.97916666662908"],[1672987200,"91.09730848845939"],[1672987500,"88.42105263178856"],[1672987800,"91.45833333326576"],[1672988100,"90.67357512958837"],[1672988400,"74.81713688631181"]]}],"ResultType":"matrix","Error":null}`

	var expected map[string]interface{}
	if err = json.Unmarshal([]byte(expectedJson), &expected); err != nil {
		t.Fatal(err)
	}

	type fields struct {
		Cfg     AdaptorConfig
		Client  client.Client
		Factory *QueryCommandRunnerFactory
	}
	type args struct {
		ctx context.Context
		cmd applications.PromCommand
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
				Cfg: AdaptorConfig{
					Timeout: MustParseDuration("1m", t),
					Verbose: true,
				},
				Client:  mockClient,
				Factory: queryCommandRunnerFactory,
			},
			args: args{
				ctx: context.Background(),
				cmd: applications.PromCommand{
					Cmd:           `max_over_time(cpu{host=~"tele.*"}[5m])`,
					Database:      "telegraf",
					Start:         &startTime2,
					End:           &endTime2,
					Timezone:      timezone,
					Evaluation:    nil,
					Step:          0,
					DataType:      applications.GRAPH_DATA,
					ValueFieldKey: "usage_idle",
				},
			},
			want:    expected,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receiver := &QueryCommandRunner{
				Cfg:     tt.fields.Cfg,
				Client:  tt.fields.Client,
				Factory: tt.fields.Factory,
			}
			got, err := receiver.Run(tt.args.ctx, tt.args.cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			got = got.(RunResult)
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

func TestQueryCommandRunner_Run_Grafana_Datasource_Test(t *testing.T) {
	expectedJson := `{"Result":[1672988400,"2"],"ResultType":"scalar","Error":null}`
	var expected map[string]interface{}
	if err := json.Unmarshal([]byte(expectedJson), &expected); err != nil {
		t.Fatal(err)
	}

	type fields struct {
		Cfg     AdaptorConfig
		Client  client.Client
		Factory *QueryCommandRunnerFactory
	}
	type args struct {
		ctx context.Context
		cmd applications.PromCommand
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
				Cfg: AdaptorConfig{
					Timeout: MustParseDuration("1m", t),
					Verbose: true,
				},
				Factory: queryCommandRunnerFactory,
			},
			args: args{
				ctx: context.Background(),
				cmd: applications.PromCommand{
					Cmd:           `1+1`,
					Database:      "prometheus",
					End:           &endTime2,
					Timezone:      timezone,
					ValueFieldKey: "usage_idle",
				},
			},
			want:    expected,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receiver := &QueryCommandRunner{
				Cfg:     tt.fields.Cfg,
				Client:  tt.fields.Client,
				Factory: tt.fields.Factory,
			}
			got, err := receiver.Run(tt.args.ctx, tt.args.cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			got = got.(RunResult)
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

func TestQueryCommandRunnerFactory_Build(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)
	receiver := &QueryCommandRunnerFactory{
		pool: sync.Pool{
			New: func() interface{} {
				return &QueryCommandRunner{}
			},
		},
	}
	runner := receiver.Build(mockClient, AdaptorConfig{})
	require.NotNil(t, runner)
}

func TestQueryCommandRunner_Recycle_Equal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)
	receiver := &QueryCommandRunnerFactory{
		pool: sync.Pool{
			New: func() interface{} {
				return &QueryCommandRunner{}
			},
		},
	}
	runner := receiver.Build(mockClient, AdaptorConfig{})
	runner.Recycle()
	runner1 := receiver.Build(mockClient, AdaptorConfig{})
	require.Equal(t, fmt.Sprintf("%p", runner), fmt.Sprintf("%p", runner1))
}

func TestQueryCommandRunner_Recycle_NotEqual(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)
	receiver := &QueryCommandRunnerFactory{
		pool: sync.Pool{
			New: func() interface{} {
				return &QueryCommandRunner{}
			},
		},
	}
	runner := receiver.Build(mockClient, AdaptorConfig{})
	runner1 := receiver.Build(mockClient, AdaptorConfig{})
	require.NotEqual(t, fmt.Sprintf("%p", runner), fmt.Sprintf("%p", runner1))
}

func TestQueryCommandRunner_Run_LabelValuesEmptyResult(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	database := "prometheus"
	influxCmd := "SHOW TAG VALUES ON prometheus FROM go_goroutines WITH KEY = job WHERE time <= '2023-01-06T07:00:00Z'"

	var response client.Response
	testFile := filepath.Join(testDir, "querycommandrunner_test_response4.json")
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

	expectedJson := `{"Result":null,"ResultType":"","Error":null}`

	var expected map[string]interface{}
	if err = json.Unmarshal([]byte(expectedJson), &expected); err != nil {
		t.Fatal(err)
	}

	type fields struct {
		Cfg     AdaptorConfig
		Client  client.Client
		Factory *QueryCommandRunnerFactory
	}
	type args struct {
		ctx context.Context
		cmd applications.PromCommand
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
				Cfg: AdaptorConfig{
					Timeout: MustParseDuration("1m", t),
					Verbose: true,
				},
				Client:  mockClient,
				Factory: queryCommandRunnerFactory,
			},
			args: args{
				ctx: context.Background(),
				cmd: applications.PromCommand{
					Cmd:       `go_goroutines`,
					Database:  "prometheus",
					Start:     nil,
					End:       &endTime2,
					Timezone:  timezone,
					DataType:  applications.LABEL_VALUES_DATA,
					LabelName: "job",
				},
			},
			want:    expected,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receiver := &QueryCommandRunner{
				Cfg:     tt.fields.Cfg,
				Client:  tt.fields.Client,
				Factory: tt.fields.Factory,
			}
			got, err := receiver.Run(tt.args.ctx, tt.args.cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			got = got.(RunResult)
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

func TestQueryCommandRunner_Run_LabelValues(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	database := "prometheus"
	influxCmd := "SHOW TAG VALUES ON prometheus FROM go_goroutines WITH KEY = job WHERE time <= '2023-01-15T04:00:00Z'"

	var response client.Response
	testFile := filepath.Join(testDir, "querycommandrunner_test_response5.json")
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

	expectedJson := `{"Result":["node","promql2influxql_promql2influxql"],"ResultType":"","Error":null}`

	var expected map[string]interface{}
	if err = json.Unmarshal([]byte(expectedJson), &expected); err != nil {
		t.Fatal(err)
	}

	type fields struct {
		Cfg     AdaptorConfig
		Client  client.Client
		Factory *QueryCommandRunnerFactory
	}
	type args struct {
		ctx context.Context
		cmd applications.PromCommand
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
				Cfg: AdaptorConfig{
					Timeout: MustParseDuration("1m", t),
					Verbose: true,
				},
				Client:  mockClient,
				Factory: queryCommandRunnerFactory,
			},
			args: args{
				ctx: context.Background(),
				cmd: applications.PromCommand{
					Cmd:       `go_goroutines`,
					Database:  "prometheus",
					Timezone:  timezone,
					DataType:  applications.LABEL_VALUES_DATA,
					LabelName: "job",
					End:       &endTime3,
				},
			},
			want:    expected,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receiver := &QueryCommandRunner{
				Cfg:     tt.fields.Cfg,
				Client:  tt.fields.Client,
				Factory: tt.fields.Factory,
			}
			got, err := receiver.Run(tt.args.ctx, tt.args.cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			got = got.(RunResult)
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

func TestQueryCommandRunner_Run_LabelValuesNoCmd(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	database := "prometheus"
	influxCmd := "SHOW TAG VALUES ON prometheus WITH KEY = job"

	var response client.Response
	testFile := filepath.Join(testDir, "querycommandrunner_test_response6.json")
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

	expectedJson := `{"Result":["node","promql2influxql_promql2influxql"],"ResultType":"","Error":null}`

	var expected map[string]interface{}
	if err = json.Unmarshal([]byte(expectedJson), &expected); err != nil {
		t.Fatal(err)
	}

	type fields struct {
		Cfg     AdaptorConfig
		Client  client.Client
		Factory *QueryCommandRunnerFactory
	}
	type args struct {
		ctx context.Context
		cmd applications.PromCommand
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
				Cfg: AdaptorConfig{
					Timeout: MustParseDuration("1m", t),
					Verbose: true,
				},
				Client:  mockClient,
				Factory: queryCommandRunnerFactory,
			},
			args: args{
				ctx: context.Background(),
				cmd: applications.PromCommand{
					Database:  "prometheus",
					Timezone:  timezone,
					DataType:  applications.LABEL_VALUES_DATA,
					LabelName: "job",
					End:       &endTime3,
				},
			},
			want:    expected,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receiver := &QueryCommandRunner{
				Cfg:     tt.fields.Cfg,
				Client:  tt.fields.Client,
				Factory: tt.fields.Factory,
			}
			got, err := receiver.Run(tt.args.ctx, tt.args.cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("Run() error = %+v, wantErr %v", err, tt.wantErr)
				return
			}
			got = got.(RunResult)
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
