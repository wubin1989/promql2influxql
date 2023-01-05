package promql

import (
	"github.com/influxdata/flux"
	influxdb "github.com/influxdata/influxdb1-client/v2"
	"github.com/prometheus/prometheus/promql/parser"
)

func isErrorTable(tbl flux.Table) bool {
	cols := tbl.Cols()
	return len(cols) == 2 && cols[0].Label == "error" && cols[1].Label == "reference"
}

func InfluxResultToPromQLValue(result []influxdb.Result, valType parser.ValueType) (parser.Value, error) {

}
