package transpiler

import (
	"github.com/influxdata/influxql"
	"github.com/pkg/errors"
	"github.com/prometheus/prometheus/promql/parser"
)

type aggregateFn struct {
	name string
	// drop tags because InfluxDB error: mixing aggregate and non-aggregate queries is not supported
	dropTag bool
}

var aggregateFns = map[parser.ItemType]aggregateFn{
	parser.SUM:      {name: "sum", dropTag: true},
	parser.AVG:      {name: "mean", dropTag: true},
	parser.MAX:      {name: "max"},
	parser.MIN:      {name: "min"},
	parser.COUNT:    {name: "count", dropTag: true},
	parser.STDDEV:   {name: "stddev", dropTag: true},
	parser.TOPK:     {name: "top"},
	parser.BOTTOMK:  {name: "bottom"},
	parser.QUANTILE: {name: "percentile"}, // TODO add unit tests
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
	if t.Start != nil {

	} else {
		if len(dimensions) > 0 {
			statement.Dimensions = dimensions
		}
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
			aggArgs = append(aggArgs, &influxql.IntegerLiteral{
				Val: int64(lit.Val),
			})
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
	defer func() {
		t.aggregateLevel++
	}()
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
			if t.aggregateLevel > 0 {
				var selectStatement influxql.SelectStatement
				selectStatement.Sources = []influxql.Source{
					&influxql.SubQuery{
						Statement: statement,
					},
				}
				field := &influxql.Field{
					Expr: &influxql.VarRef{
						Val: statement.Fields[len(statement.Fields)-1].Name(),
					},
				}
				t.setAggregateFields(&selectStatement, field, parameter, aggFn)
				t.setAggregateDimension(&selectStatement, a.Grouping...)
				return &selectStatement, nil
			}
			t.setAggregateFields(statement, statement.Fields[len(statement.Fields)-1], parameter, aggFn)
			t.setAggregateDimension(statement, a.Grouping...)
			statement.Limit = 0
		default:
			return nil, ErrPromExprNotSupported
		}
	default:
		return nil, ErrPromExprNotSupported
	}
	return expr, nil
}
