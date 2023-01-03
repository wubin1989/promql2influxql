package promql2influxql

import (
	_ "github.com/influxdata/influxdb1-client" // this is important because of the bug in go mod
	client "github.com/influxdata/influxdb1-client/v2"
	"github.com/pkg/errors"
	"time"
)

type DialectType int

const (
	PROMQL DialectType = iota + 1
	INFLUXQL
)

type Command struct {
	Cmd     string
	Dialect DialectType
}

type InfluxDBAdaptorConfig struct {
	HTTPConfig client.HTTPConfig
}

func assertInfluxDBAdaptorNotNil(adaptor *InfluxDBAdaptor) {
	if adaptor == nil {
		panic("please create InfluxDBAdaptor first")
	}
}

var _ client.Client = (*InfluxDBAdaptor)(nil)
var _ IAdaptor = (*InfluxDBAdaptor)(nil)

type InfluxDBAdaptor struct {
	_              [0]int
	Cfg            InfluxDBAdaptorConfig
	InfluxDBClient client.Client
}

func (receiver *InfluxDBAdaptor) Ping(timeout time.Duration) (time.Duration, string, error) {
	assertInfluxDBAdaptorNotNil(receiver)
	return receiver.InfluxDBClient.Ping(timeout)
}

func (receiver *InfluxDBAdaptor) Write(bp client.BatchPoints) error {
	assertInfluxDBAdaptorNotNil(receiver)
	return receiver.InfluxDBClient.Write(bp)
}

func (receiver *InfluxDBAdaptor) Query(q client.Query) (*client.Response, error) {
	assertInfluxDBAdaptorNotNil(receiver)
	return receiver.InfluxDBClient.Query(q)
}

func (receiver *InfluxDBAdaptor) QueryAsChunk(q client.Query) (*client.ChunkedResponse, error) {
	assertInfluxDBAdaptorNotNil(receiver)
	return receiver.InfluxDBClient.QueryAsChunk(q)
}

func (receiver *InfluxDBAdaptor) Close() error {
	assertInfluxDBAdaptorNotNil(receiver)
	return receiver.InfluxDBClient.Close()
}

func NewInfluxDBAdaptor(cfg InfluxDBAdaptorConfig) *InfluxDBAdaptor {
	adaptor := InfluxDBAdaptor{
		Cfg: cfg,
	}
	return &adaptor
}

func (receiver *InfluxDBAdaptor) Initialize() {
	assertInfluxDBAdaptorNotNil(receiver)
	cfg := receiver.Cfg
	c, err := client.NewHTTPClient(cfg.HTTPConfig)
	if err != nil {
		panic(errors.Wrapf(err, "Error creating InfluxDB Client: "))
	}
	receiver.InfluxDBClient = c
}

func (receiver *InfluxDBAdaptor) ToInfluxQL() string {
	return ""
}
