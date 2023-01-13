package transpiler

import (
	"github.com/influxdata/influxql"
	"github.com/pkg/errors"
	"github.com/prometheus/prometheus/promql/parser"
	influx "github.com/wubin1989/promql2influxql/influxql"
)

var aggregateOverTimeFns = map[string]aggregateFn{
	"sum_over_time": {
		name:         "sum",
		dropTag:      true,
		functionType: influx.AGGREGATE_FN,
	},
	"avg_over_time": {
		name:         "mean",
		dropTag:      true,
		functionType: influx.AGGREGATE_FN,
	},
	"max_over_time": {
		name:         "max",
		dropTag:      false,
		functionType: influx.SELECTOR_FN,
	},
	"min_over_time": {
		name:         "min",
		dropTag:      false,
		functionType: influx.SELECTOR_FN,
	},
	"count_over_time": {
		name:         "count",
		dropTag:      true,
		functionType: influx.AGGREGATE_FN,
	},
	"stddev_over_time": {
		name:         "stddev",
		dropTag:      true,
		functionType: influx.AGGREGATE_FN,
	},
	"quantile_over_time": {
		name:         "percentile",
		dropTag:      false,
		functionType: influx.SELECTOR_FN,
	},
}

var vectorMathFunctions = map[string]aggregateFn{
	"abs": {
		name:         "abs",
		dropTag:      false,
		functionType: influx.TRANSFORM_FN,
	},
	"ceil": {
		name:         "ceil",
		dropTag:      false,
		functionType: influx.TRANSFORM_FN,
	},
	"floor": {
		name:         "floor",
		dropTag:      false,
		functionType: influx.TRANSFORM_FN,
	},
	"exp": {
		name:         "exp",
		dropTag:      false,
		functionType: influx.TRANSFORM_FN,
	},
	"sqrt": {
		name:         "sqrt",
		dropTag:      false,
		functionType: influx.TRANSFORM_FN,
	},
	"ln": {
		name:         "log",
		dropTag:      false,
		functionType: influx.TRANSFORM_FN,
	},
	"log2": {
		name:         "log2",
		dropTag:      false,
		functionType: influx.TRANSFORM_FN,
	},
	"log10": {
		name:         "log10",
		dropTag:      false,
		functionType: influx.TRANSFORM_FN,
	},
	"round": {
		name:         "round",
		dropTag:      false,
		functionType: influx.TRANSFORM_FN,
	},
	"acos": {
		name:         "acos",
		dropTag:      false,
		functionType: influx.TRANSFORM_FN,
	},
	"asin": {
		name:         "asin",
		dropTag:      false,
		functionType: influx.TRANSFORM_FN,
	},
	"atan": {
		name:         "atan",
		dropTag:      false,
		functionType: influx.TRANSFORM_FN,
	},
	"cos": {
		name:         "cos",
		dropTag:      false,
		functionType: influx.TRANSFORM_FN,
	},
	"sin": {
		name:         "sin",
		dropTag:      false,
		functionType: influx.TRANSFORM_FN,
	},
	"tan": {
		name:         "tan",
		dropTag:      false,
		functionType: influx.TRANSFORM_FN,
	},
	"rate": {
		name:         "non_negative_derivative",
		dropTag:      false,
		functionType: influx.TRANSFORM_FN,
	},
	"deriv": {
		name:         "derivative",
		dropTag:      false,
		functionType: influx.TRANSFORM_FN,
	},
}

func (t *Transpiler) transpileVectorMathFunc(aggFn aggregateFn, inArgs []influxql.Node) (influxql.Node, error) {
	return t.transpileAggregateOverTimeFunc(aggFn, inArgs)
}

func (t *Transpiler) transpileAggregateOverTimeFunc(aggFn aggregateFn, inArgs []influxql.Node) (influxql.Node, error) {
	table := inArgs[len(inArgs)-1]
	var parameter influxql.Expr
	if aggFn.name == "percentile" {
		parameter = inArgs[0].(influxql.Expr)
	}
	switch n := table.(type) {
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
				return &selectStatement, nil
			default:
				t.setAggregateFields(statement, field, parameter, aggFn)
			}
		default:
			return nil, ErrPromExprNotSupported
		}
	default:
		return nil, ErrPromExprNotSupported
	}
	return table, nil
}

// transpileCall transpiles PromQL Call expression
func (t *Transpiler) transpileCall(a *parser.Call) (influxql.Node, error) {
	// The PromQL parser already verifies argument counts and types, so we don't have to check this here.
	args := make([]influxql.Node, len(a.Args))
	for i, arg := range a.Args {
		tArg, err := t.transpileExpr(arg)
		if err != nil {
			return nil, errors.Errorf("error transpiling function argument: %s", err)
		}
		args[i] = tArg
	}

	// {count,avg,sum,min,max,...}_over_time()
	if fn, ok := aggregateOverTimeFns[a.Func.Name]; ok {
		return t.transpileAggregateOverTimeFunc(fn, args)
	}

	if fn, ok := vectorMathFunctions[a.Func.Name]; ok {
		return t.transpileVectorMathFunc(fn, args)
	}

	return nil, ErrPromExprNotSupported
}
