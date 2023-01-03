package promql2influxql

import client "github.com/influxdata/influxdb1-client/v2"

type IAdaptor interface {
	client.Client
}
