package promql

import (
	"encoding/json"
	"github.com/influxdata/influxdb1-client/models"
	influxdb "github.com/influxdata/influxdb1-client/v2"
	"github.com/influxdata/influxql"
	"github.com/pkg/errors"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/timestamp"
	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/promql/parser"
	"github.com/unionj-cloud/go-doudou/v2/toolkit/caller"
	"github.com/unionj-cloud/go-doudou/v2/toolkit/stringutils"
	"github.com/wubin1989/promql2influxql/command"
	"sort"
	"time"
)

// InfluxLiteralToPromQLValue converts influxql.Literal expression to parser.Value of Prometheus
func (receiver *QueryCommandRunner) InfluxLiteralToPromQLValue(result influxql.Literal, cmd command.Command) (value parser.Value, resultType string) {
	now := time.Now()
	if cmd.Evaluation != nil {
		now = *cmd.Evaluation
	} else if cmd.End != nil {
		now = *cmd.End
	}
	switch lit := result.(type) {
	case *influxql.NumberLiteral:
		return promql.Scalar{
			T: timestamp.FromTime(now),
			V: lit.Val,
		}, string(parser.ValueTypeScalar)
	case *influxql.IntegerLiteral:
		return promql.Scalar{
			T: timestamp.FromTime(now),
			V: float64(lit.Val),
		}, string(parser.ValueTypeScalar)
	default:
		return promql.String{
			T: timestamp.FromTime(now),
			V: lit.String(),
		}, string(parser.ValueTypeString)
	}
}

// populatePromSeries populates *promql.Series slice from models.Row returned by InfluxDB
func (receiver *QueryCommandRunner) populatePromSeries(promSeries *[]*promql.Series, item models.Row) error {
	metric := labels.FromMap(item.Tags)
	metric = append(metric, labels.FromStrings("__name__", item.Name)...)
	var points []promql.Point
	for _, item1 := range item.Values {
		ts, err := time.Parse(time.RFC3339Nano, item1[0].(string))
		if err != nil {
			return errors.Wrap(err, "parse time fail")
		}
		point := promql.Point{
			T: timestamp.FromTime(ts),
		}
		switch number := item1[len(item1)-1].(type) {
		case json.Number:
			if v, err := number.Float64(); err == nil {
				point.V = v
			} else {
				if v, err := number.Int64(); err == nil {
					point.V = float64(v)
				}
			}
		case float64:
			point.V = number
		}
		points = append(points, point)
	}
	*promSeries = append(*promSeries, &promql.Series{
		Metric: metric,
		Points: points,
	})
	return nil
}

// populateSeriesMap populates series map seriesMap with hash of labels.Labels as map key
// and *promql.Series as map value from models.Row returned from InfluxDB
func (receiver *QueryCommandRunner) populateSeriesMap(seriesMap map[uint64]*promql.Series, table models.Row) error {
	for _, row := range table.Values {
		kvs := make(map[string]string)
		for i, col := range row {
			if i == 0 || i == len(row)-1 {
				continue
			}
			kvs[table.Columns[i]] = col.(string)
		}
		metric := labels.FromMap(kvs)
		metric = append(metric, labels.FromStrings("__name__", table.Name)...)

		ts, err := time.Parse(time.RFC3339Nano, row[0].(string))
		if err != nil {
			return errors.Wrap(err, "parse time fail")
		}
		point := promql.Point{
			T: timestamp.FromTime(ts),
		}
		switch number := row[len(row)-1].(type) {
		case json.Number:
			if v, err := number.Float64(); err == nil {
				point.V = v
			} else {
				if v, err := number.Int64(); err == nil {
					point.V = float64(v)
				}
			}
		case float64:
			point.V = number
		}

		if series, exists := seriesMap[metric.Hash()]; exists {
			series.Points = append(series.Points, point)
		} else {
			seriesMap[metric.Hash()] = &promql.Series{
				Metric: metric,
				Points: []promql.Point{
					point,
				},
			}
		}
	}
	return nil
}

// populateSeriesSlice populates *promql.Series slice from series map seriesMap
func (receiver *QueryCommandRunner) populateSeriesSlice(promSeries *[]*promql.Series, seriesMap map[uint64]*promql.Series, table models.Row) {
	m := make(map[*promql.Series]struct{})
	for _, row := range table.Values {
		kvs := make(map[string]string)
		for i, col := range row {
			if i == 0 || i == len(row)-1 {
				continue
			}
			kvs[table.Columns[i]] = col.(string)
		}
		metric := labels.FromMap(kvs)
		metric = append(metric, labels.FromStrings("__name__", table.Name)...)
		series := seriesMap[metric.Hash()]
		if _, exists := m[series]; !exists {
			*promSeries = append(*promSeries, series)
			m[series] = struct{}{}
		}
	}
}

// groupResultBySeries is used to populate *promql.Series slice from models.Row returned from InfluxDB
// when raw result has not grouped by series(measurement + tag key/value pairs).
func (receiver *QueryCommandRunner) groupResultBySeries(promSeries *[]*promql.Series, table models.Row) error {
	// 1. Iterate the whole result table to collect all series into seriesMap. The map key is hash of label set, the map value is
	// a pointer to promql.Series. Each series may contain one or more points.
	seriesMap := make(map[uint64]*promql.Series)
	if err := receiver.populateSeriesMap(seriesMap, table); err != nil {
		return errors.Wrap(err, caller.NewCaller().String())
	}

	// 2. We iterate the whole result table again in order to append each series to promSeries while keep the same order
	// as in the result table
	receiver.populateSeriesSlice(promSeries, seriesMap, table)
	return nil
}

// InfluxResultToPromQLValue converts influxdb.Result slice to parser.Value of Prometheus
func (receiver *QueryCommandRunner) InfluxResultToPromQLValue(results []influxdb.Result, expr parser.Expr, cmd command.Command) (value parser.Value, resultType string, err error) {
	if len(results) == 0 {
		return nil, "", nil
	}
	result := results[0]
	if stringutils.IsNotEmpty(result.Err) {
		return nil, "", errors.New(result.Err)
	}
	var promSeries []*promql.Series
	for _, item := range result.Series {
		if len(item.Tags) > 0 {
			if err := receiver.populatePromSeries(&promSeries, item); err != nil {
				return nil, "", errors.Wrap(err, "error from populatePromSeries")
			}
		} else {
			if err := receiver.groupResultBySeries(&promSeries, item); err != nil {
				return nil, "", errors.Wrap(err, "error from populatePromSeries")
			}
		}
	}
	switch expr.Type() {
	case parser.ValueTypeMatrix:
		return receiver.handleValueTypeMatrix(promSeries), string(parser.ValueTypeMatrix), nil
	case parser.ValueTypeVector:
		switch cmd.DataType {
		case command.GRAPH_DATA:
			return receiver.handleValueTypeMatrix(promSeries), string(parser.ValueTypeMatrix), nil
		default:
			value, err = receiver.handleValueTypeVector(promSeries)
			return value, string(parser.ValueTypeVector), err
		}
	default:
		return nil, "", errors.Errorf("unsupported PromQL value type: %s", expr.Type())
	}
}

// InfluxResultToStringSlice converts influxdb.Result slice to string slice
func (receiver *QueryCommandRunner) InfluxResultToStringSlice(results []influxdb.Result, dest *[]string, expr parser.Expr, cmd command.Command) error {
	if len(results) == 0 {
		return nil
	}
	result := results[0]
	if stringutils.IsNotEmpty(result.Err) {
		return errors.New(result.Err)
	}
	if len(result.Series) == 0 {
		return nil
	}
	tagValueMap := make(map[string]struct{})
	for _, item := range result.Series {
		for _, item1 := range item.Values {
			if len(item1) <= 1 {
				continue
			}
			tagValue := item1[1].(string)
			if _, exists := tagValueMap[tagValue]; exists {
				continue
			} else {
				tagValueMap[tagValue] = struct{}{}
			}
			*dest = append(*dest, tagValue)
		}
	}
	return nil
}

func (receiver *QueryCommandRunner) handleValueTypeMatrix(promSeries []*promql.Series) promql.Matrix {
	matrix := make(promql.Matrix, 0, len(promSeries))
	for _, ser := range promSeries {
		matrix = append(matrix, *ser)
	}
	sort.Sort(matrix)
	return matrix
}

func (receiver *QueryCommandRunner) handleValueTypeVector(promSeries []*promql.Series) (promql.Vector, error) {
	vector := make(promql.Vector, 0, len(promSeries))
	for _, ser := range promSeries {
		if len(ser.Points) != 1 {
			return nil, errors.Errorf("expected exactly one output point for every series for vector result")
		}
		vector = append(vector, promql.Sample{
			Metric: ser.Metric,
			Point:  ser.Points[0],
		})
	}
	return vector, nil
}
