package transpiler

import (
	"github.com/influxdata/influxql"
	"github.com/pkg/errors"
	"github.com/prometheus/prometheus/promql/parser"
	influx "github.com/wubin1989/promql2influxql/influxql"
)

type aggregateFn struct {
	name string
	// drop tags because InfluxDB error: mixing aggregate and non-aggregate queries is not supported
	dropTag                bool
	functionType           influx.FunctionType
	expectIntegerParameter bool
}

var aggregateFns = map[parser.ItemType]aggregateFn{
	parser.SUM:      {name: "sum", dropTag: true, functionType: influx.AGGREGATE_FN},
	parser.AVG:      {name: "mean", dropTag: true, functionType: influx.AGGREGATE_FN},
	parser.MAX:      {name: "max", functionType: influx.SELECTOR_FN},
	parser.MIN:      {name: "min", functionType: influx.SELECTOR_FN},
	parser.COUNT:    {name: "count", dropTag: true, functionType: influx.AGGREGATE_FN},
	parser.STDDEV:   {name: "stddev", dropTag: true, functionType: influx.AGGREGATE_FN},
	parser.TOPK:     {name: "top", functionType: influx.SELECTOR_FN, expectIntegerParameter: true},
	parser.BOTTOMK:  {name: "bottom", functionType: influx.SELECTOR_FN, expectIntegerParameter: true},
	parser.QUANTILE: {name: "percentile", functionType: influx.SELECTOR_FN}, // TODO add unit tests
}

func columnList(dimensions *[]*influxql.Dimension, strs ...string) {
	for _, str := range strs {
		*dimensions = append(*dimensions, &influxql.Dimension{
			Expr: &influxql.VarRef{Val: str},
		})
	}
}

func (t *Transpiler) setAggregateDimension(statement *influxql.SelectStatement, grouping ...string) {
	dimensions := make([]*influxql.Dimension, 0, len(grouping))
	columnList(&dimensions, grouping...)
	if len(dimensions) > 0 {
		statement.Dimensions = dimensions
	}
}

func (t *Transpiler) setAggregateFields(selectStatement *influxql.SelectStatement, field *influxql.Field, parameter influxql.Expr, aggFn aggregateFn) {
	var fields []*influxql.Field
	if !aggFn.dropTag {
		fields = append(fields, &influxql.Field{
			Expr: &influxql.Wildcard{
				Type: influxql.TAG,
			},
		})
	} else {
		t.tagDropped = true
	}
	aggArgs := []influxql.Expr{
		field.Expr,
	}
	if parameter != nil {
		if lit, ok := parameter.(*influxql.NumberLiteral); ok {
			if aggFn.expectIntegerParameter {
				aggArgs = append(aggArgs, &influxql.IntegerLiteral{
					Val: int64(lit.Val),
				})
			} else {
				aggArgs = append(aggArgs, &influxql.NumberLiteral{
					Val: lit.Val,
				})
			}
		} else {
			aggArgs = append(aggArgs, parameter)
		}
	}
	fields = append(fields, &influxql.Field{
		Expr: &influxql.Call{Name: aggFn.name, Args: aggArgs},
	})
	selectStatement.Fields = fields
}

func (t *Transpiler) transpileAggregateExpr(a *parser.AggregateExpr) (influxql.Node, error) {
	expr, err := t.transpileExpr(a.Expr)
	if err != nil {
		return nil, errors.Errorf("error transpiling aggregate sub-expression: %s", err)
	}
	if a.Without {
		return nil, errors.New("unsupported aggregate operator: without")
	}
	var parameter influxql.Expr
	if a.Param != nil {
		if !yieldsFloat(a.Param) {
			return nil, errors.Errorf("only support yielding float parameter sub-expression in aggregate expression")
		}
		param, err := t.transpileExpr(a.Param)
		if err != nil {
			return nil, errors.Errorf("error transpiling aggregate parameter sub-expression: %s", err)
		}
		parameter = param.(influxql.Expr)
	}
	aggFn, ok := aggregateFns[a.Op]
	if !ok {
		return nil, errors.Errorf("unsupported aggregation type %s", a.Op)
	}
	switch n := expr.(type) {
	case influxql.Statement:
		switch statement := n.(type) {
		case *influxql.SelectStatement:
			field := statement.Fields[len(statement.Fields)-1]
			switch field.Expr.(type) {
			case *influxql.Call:
				var selectStatement influxql.SelectStatement
				selectStatement.Sources = []influxql.Source{
					&influxql.SubQuery{
						Statement: statement,
					},
				}
				wrappedField := &influxql.Field{
					Expr: &influxql.VarRef{
						Val: field.Name(),
					},
				}
				t.setAggregateFields(&selectStatement, wrappedField, parameter, aggFn)
				t.setAggregateDimension(&selectStatement, a.Grouping...)
				return &selectStatement, nil
			default:
				t.setAggregateFields(statement, field, parameter, aggFn)
				t.setAggregateDimension(statement, a.Grouping...)
			}
		default:
			return nil, ErrPromExprNotSupported
		}
	default:
		return nil, ErrPromExprNotSupported
	}
	return expr, nil
}
