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
	"github.com/unionj-cloud/go-doudou/v2/toolkit/stringutils"
	"github.com/wubin1989/promql2influxql/command"
	"sort"
	"time"
)

func (receiver *QueryCommandRunner) InfluxLiteralToPromQLValue(result influxql.Literal) parser.Value {
	now := time.Now()
	switch lit := result.(type) {
	case *influxql.NumberLiteral:
		return promql.Scalar{
			T: timestamp.FromTime(now),
			V: lit.Val,
		}
	case *influxql.IntegerLiteral:
		return promql.Scalar{
			T: timestamp.FromTime(now),
			V: float64(lit.Val),
		}
	default:
		return promql.String{
			T: timestamp.FromTime(now),
			V: lit.String(),
		}
	}
}

func (receiver QueryCommandRunner) groupByResult(promSeries *[]*promql.Series, item models.Row) error {
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

type SeriesKeyValue struct {
	SeriesKey models.Tags
	Value     float64
}

func (receiver QueryCommandRunner) groupResultBySeries(promSeries *[]*promql.Series, table models.Row) error {
	// 1. Iterate the whole result table to collect all series into seriesMap. The map key is hash of label set, the map value is
	// a pointer to promql.Series. Each series may contain one or more points.
	seriesMap := make(map[uint64]*promql.Series)
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

	// 2. We iterate the whole result table again in order to append each series to promSeries while keep the same order
	// as in the result table
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
	return nil
}

func (receiver *QueryCommandRunner) InfluxResultToPromQLValue(results []influxdb.Result, expr parser.Expr, cmd command.Command) (parser.Value, error) {
	if len(results) == 0 {
		return nil, nil
	}
	result := results[0]
	if stringutils.IsNotEmpty(result.Err) {
		return nil, errors.New(result.Err)
	}
	var promSeries []*promql.Series
	for _, item := range result.Series {
		if len(item.Tags) > 0 {
			if err := receiver.groupByResult(&promSeries, item); err != nil {
				return nil, errors.Wrap(err, "error from groupByResult")
			}
		} else {
			if err := receiver.groupResultBySeries(&promSeries, item); err != nil {
				return nil, errors.Wrap(err, "error from groupByResult")
			}
		}
	}
	switch expr.Type() {
	case parser.ValueTypeMatrix:
		return receiver.handleValueTypeMatrix(promSeries), nil
	case parser.ValueTypeVector:
		switch cmd.DataType {
		case command.GRAPH_DATA:
			return receiver.handleValueTypeMatrix(promSeries), nil
		default:
			return receiver.handleValueTypeVector(promSeries)
		}
	default:
		return nil, errors.Errorf("unsupported PromQL value type: %s", expr.Type())
	}
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
