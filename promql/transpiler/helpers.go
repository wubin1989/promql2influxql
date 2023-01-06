package transpiler

//
//import (
//	"fmt"
//	"github.com/prometheus/prometheus/model/labels"
//	"github.com/prometheus/prometheus/promql/parser"
//	"sort"
//
//	"github.com/influxdata/flux"
//	"github.com/influxdata/flux/execute"
//	"github.com/influxdata/flux/semantic"
//)
//
//func isErrorTable(tbl flux.Table) bool {
//	cols := tbl.Cols()
//	return len(cols) == 2 && cols[0].Label == "error" && cols[1].Label == "reference"
//}
//
//// FluxResultToPromQLValue translates a Flux result to a PromQL value
//// of the desired type. For range query results, the passed-in value type
//// should always be parser.ValueTypeMatrix (even if the root node for a
//// range query can be of scalar type, range queries always return matrices).
//func FluxResultToPromQLValue(result flux.Result, valType parser.ValueType) (parser.Value, error) {
//	hashToSeries := map[uint64]*parser.Series{}
//
//	err := result.Tables().Do(func(tbl flux.Table) error {
//		if isErrorTable(tbl) {
//			return errors.Errorf("flux error: %s", tbl.Key().ValueString(0))
//		}
//
//		tbl.Do(func(cr flux.ColReader) error {
//			// Each row corresponds to one PromQL metric / series.
//			for i := 0; i < cr.Len(); i++ {
//				builder := labels.NewBuilder(nil)
//				var val float64
//				var ts int64
//
//				// Extract PromQL labels and timestamp/value from the columns.
//				for j, col := range cr.Cols() {
//					switch col.Label {
//					case execute.DefaultTimeColLabel:
//						ts = execute.ValueForRow(cr, i, j).Time().Time().UnixNano() / 1e6
//					case execute.DefaultValueColLabel:
//						v := execute.ValueForRow(cr, i, j)
//						switch nat := v.Type().Nature(); nat {
//						case semantic.Float:
//							val = v.Float()
//						default:
//							return errors.Errorf("invalid column value type: %s", nat.String())
//						}
//					case execute.DefaultStartColLabel, execute.DefaultStopColLabel, "_measurement":
//						// Ignore.
//						// Window boundaries are only interesting within the Flux pipeline.
//						// _measurement is always set to the constant "prometheus" for now.
//					default:
//						ln := UnescapeLabelName(col.Label)
//						builder.Set(ln, cr.Strings(j).ValueString(i))
//					}
//				}
//
//				lbls := builder.Labels()
//				point := parser.Point{
//					T: ts,
//					V: val,
//				}
//				hash := lbls.Hash()
//				if ser, ok := hashToSeries[hash]; !ok {
//					hashToSeries[hash] = &parser.Series{
//						Metric: lbls,
//						Points: []parser.Point{point},
//					}
//				} else {
//					ser.Points = append(ser.Points, point)
//				}
//			}
//			return nil
//		})
//		return nil
//	})
//
//	if err != nil {
//		return nil, err
//	}
//
//	switch valType {
//	case parser.ValueTypeMatrix:
//		matrix := make(parser.Matrix, 0, len(hashToSeries))
//		for _, ser := range hashToSeries {
//			matrix = append(matrix, *ser)
//		}
//		sort.Sort(matrix)
//		return matrix, nil
//
//	case parser.ValueTypeVector:
//		vector := make(parser.Vector, 0, len(hashToSeries))
//		for _, ser := range hashToSeries {
//			if len(ser.Points) != 1 {
//				return nil, errors.Errorf("expected exactly one output point for every series for vector result")
//			}
//			vector = append(vector, parser.Sample{
//				Metric: ser.Metric,
//				Point:  ser.Points[0],
//			})
//		}
//		// TODO: Implement sorting for vectors, but this is only needed for tests.
//		// sort.Sort(vector)
//		return vector, nil
//
//	case parser.ValueTypeScalar:
//		if len(hashToSeries) != 1 {
//			return nil, errors.Errorf("expected exactly one output series for scalar result")
//		}
//		for _, ser := range hashToSeries {
//			if len(ser.Points) != 1 {
//				return nil, errors.Errorf("expected exactly one output point for scalar result")
//			}
//			return parser.Scalar{
//				T: ser.Points[0].T,
//				V: ser.Points[0].V,
//			}, nil
//		}
//		// Should be unreachable due to the checks above.
//		return nil, errors.Errorf("no point found")
//	default:
//		return nil, errors.Errorf("unsupported PromQL value type: %s", valType)
//	}
//}
