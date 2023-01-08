package transpiler

//
//import (
//	"github.com/influxdata/influxql"
//	"github.com/prometheus/prometheus/promql/parser"
//	"time"
//
//	"github.com/prometheus/common/model"
//)
//
//var aggregateOverTimeFns = map[string]string{
//	"sum_over_time":      "sum",
//	"avg_over_time":      "mean",
//	"max_over_time":      "max",
//	"min_over_time":      "min",
//	"count_over_time":    "count",
//	"stddev_over_time":   "stddev",
//	"quantile_over_time": "percentile",
//}
//
//var vectorMathFunctions = map[string]string{
//	"abs":   "abs",
//	"ceil":  "ceil",
//	"floor": "floor",
//	"exp":   "exp",
//	"sqrt":  "sqrt",
//	"ln":    "log",
//	"log2":  "log2",
//	"log10": "log10",
//	"round": "round",
//	"acos":  "acos",
//	"asin":  "asin",
//	"atan":  "atan",
//	"cos":   "cos",
//	"sin":   "sin",
//	"tan":   "tan",
//}
//
//var filterNullValuesCall = call(
//	"filter",
//	map[string]influxql.Expression{
//		"fn": &influxql.FunctionExpression{
//			Params: []*influxql.Property{
//				{
//					Key: &influxql.Identifier{
//						Name: "r",
//					},
//				},
//			},
//			Body: &influxql.UnaryExpression{
//				Operator: influxql.ExistsOperator,
//				Argument: member("r", "_value"),
//			},
//		},
//	},
//)
//
//// Function to apply a simple one-operand function to all values in a table.
//func singleArgFloatFn(fn string, argName string) *influxql.FunctionExpression {
//	// (r) => {r with _value: mathFn(x: r._value), _stop: r._stop}
//	return &influxql.FunctionExpression{
//		Params: []*influxql.Property{
//			{
//				Key: &influxql.Identifier{
//					Name: "r",
//				},
//			},
//		},
//		Body: &influxql.ObjectExpression{
//			With: &influxql.Identifier{Name: "r"},
//			Properties: []*influxql.Property{
//				{
//					Key: &influxql.Identifier{Name: "_value"},
//					Value: call(fn, map[string]influxql.Expression{
//						argName: member("r", "_value"),
//					}),
//				},
//				{
//					Key:   &influxql.Identifier{Name: "_stop"},
//					Value: member("r", "_stop"),
//				},
//			},
//		},
//	}
//}
//
//// Function to set all values to a constant.
//func setConstValueFn(v influxql.Expression) *influxql.FunctionExpression {
//	// (r) => {r with _value: <v>, _stop: r._stop}
//	return &influxql.FunctionExpression{
//		Params: []*influxql.Property{
//			{
//				Key: &influxql.Identifier{
//					Name: "r",
//				},
//			},
//		},
//		Body: &influxql.ObjectExpression{
//			With: &influxql.Identifier{Name: "r"},
//			Properties: []*influxql.Property{
//				{
//					Key:   &influxql.Identifier{Name: "_value"},
//					Value: v,
//				},
//				{
//					Key:   &influxql.Identifier{Name: "_stop"},
//					Value: member("r", "_stop"),
//				},
//			},
//		},
//	}
//}
//
//var filterWindowsWithZeroValueCall = call(
//	"filter",
//	map[string]influxql.Expression{
//		"fn": &influxql.FunctionExpression{
//			Params: []*influxql.Property{
//				{
//					Key: &influxql.Identifier{
//						Name: "r",
//					},
//				},
//			},
//			Body: &influxql.BinaryExpression{
//				Operator: influxql.GreaterThanOperator,
//				Left:     member("r", "_value"),
//				Right:    &influxql.FloatLiteral{Value: 0},
//			},
//		},
//	},
//)
//
//func (t *Transpiler) transpileAggregateOverTimeFunc(fn string, inArgs []influxql.Expression) (influxql.Expression, error) {
//	callFn := fn
//	vec := inArgs[0]
//	args := map[string]influxql.Expression{}
//
//	switch fn {
//	case "quantile":
//		vec = inArgs[1]
//		args["q"] = inArgs[0]
//		args["method"] = &influxql.StringLiteral{Value: "exact_mean"}
//	case "stddev", "stdvar":
//		callFn = "stddev"
//		args["mode"] = &influxql.StringLiteral{Value: "population"}
//	}
//
//	pipelineCalls := []*influxql.CallExpression{
//		call(callFn, args),
//		filterNullValuesCall,
//		call("toFloat", nil),
//		dropFieldAndTimeCall,
//	}
//
//	switch fn {
//	case "count":
//		// Count is the only function that produces a 0 instead of null value for an empty table.
//		// In PromQL, when we count_over_time() over an empty range, the result is empty, so we need
//		// to filter away 0 values here.
//		pipelineCalls = append(pipelineCalls, filterWindowsWithZeroValueCall)
//	case "stdvar":
//		pipelineCalls = append(
//			pipelineCalls,
//			call("map", map[string]influxql.Expression{
//				"fn": scalarArithBinaryMathFn("pow", &influxql.FloatLiteral{Value: 2}, false),
//			}),
//		)
//	}
//
//	return buildPipeline(
//		vec,
//		pipelineCalls...,
//	), nil
//}
//
//func labelJoinFn(srcLabels []*influxql.StringLiteral, dst *influxql.StringLiteral, sep *influxql.StringLiteral) *influxql.FunctionExpression {
//	// TODO: Deal with empty source labels! Use Flux conditionals to check for existence?
//	var dstLabelValue influxql.Expression = member("r", srcLabels[0].Value)
//	for _, srcLabel := range srcLabels[1:] {
//		dstLabelValue = &influxql.BinaryExpression{
//			Operator: influxql.AdditionOperator,
//			Left:     dstLabelValue,
//			Right: &influxql.BinaryExpression{
//				Operator: influxql.AdditionOperator,
//				Left:     sep,
//				Right:    member("r", srcLabel.Value),
//			},
//		}
//	}
//
//	// (r) => ({r with <dst>: <src1><sep><src2>...})
//	return &influxql.FunctionExpression{
//		Params: []*influxql.Property{
//			{
//				Key: &influxql.Identifier{
//					Name: "r",
//				},
//			},
//		},
//		Body: &influxql.ObjectExpression{
//			With: &influxql.Identifier{Name: "r"},
//			Properties: []*influxql.Property{
//				{
//					// This has to be a string literal and not an identifier, since
//					// it may contain special characters (like "~").
//					Key:   &influxql.StringLiteral{Value: dst.Value},
//					Value: dstLabelValue,
//				},
//				{
//					Key:   &influxql.Identifier{Name: "_value"},
//					Value: member("r", "_value"),
//				},
//			},
//		},
//	}
//}
//
//func (t *Transpiler) generateZeroWindows() *influxql.PipeExpression {
//	var windowCall *influxql.CallExpression
//	var windowFilterCall *influxql.CallExpression
//	if t.Resolution > 0 {
//		// For range queries:
//		// At every resolution step, load / look back up to 5m of data (PromQL lookback delta).
//		windowCall = call("window", map[string]influxql.Expression{
//			"every": &influxql.DurationLiteral{Values: []influxql.Duration{{Magnitude: t.Resolution.Nanoseconds(), Unit: "ns"}}},
//			// TODO: We don't actually need 5-minute windows here, as we're not looking for any actual data anyway.
//			// We just care about the window's "_stop". Should we just choose the smallest possible period, like 1ns or even 0ns?
//			"period":      &influxql.DurationLiteral{Values: []influxql.Duration{{Magnitude: 5, Unit: "m"}}},
//			"offset":      &influxql.DurationLiteral{Values: []influxql.Duration{{Magnitude: t.Start.UnixNano() % t.Resolution.Nanoseconds(), Unit: "ns"}}},
//			"createEmpty": &influxql.BooleanLiteral{Value: true},
//		})
//
//		// Remove any windows <5m long at the edges of the graph range to act like parser.
//		windowFilterCall = call("filter", map[string]influxql.Expression{"fn": windowCutoffFn(t.Start, t.End.Add(-5*time.Minute))})
//	}
//
//	return buildPipeline(
//		call("parser.emptyTable", nil),
//		call("range", map[string]influxql.Expression{
//			"start": &influxql.DateTimeLiteral{Value: t.Start.Add(-5 * time.Minute)},
//			"stop":  &influxql.DateTimeLiteral{Value: t.End},
//		}),
//		windowCall,
//		call("sum", nil),
//		windowFilterCall,
//	)
//}
//
//func (t *Transpiler) timeFn() *influxql.PipeExpression {
//	return buildPipeline(
//		t.generateZeroWindows(),
//		call("parser.timestamp", nil),
//	)
//}
//
//func (t *Transpiler) transpileCall(c *parser.Call) (influxql.Node, error) {
//	// The PromQL parser already verifies argument counts and types, so we don't have to check this here.
//	args := make([]influxql.Expression, len(c.Args))
//	for i, arg := range c.Args {
//		tArg, err := t.transpileExpr(arg)
//		if err != nil {
//			return nil, errors.Errorf("error transpiling function argument: %s", err)
//		}
//		args[i] = tArg
//	}
//
//	// {count,avg,sum,min,max,...}_over_time()
//	if fn, ok := aggregateOverTimeFns[c.Func.Name]; ok {
//		return t.transpileAggregateOverTimeFunc(fn, args)
//	}
//
//	// abs(), ceil(), round()...
//	if fn, ok := vectorMathFunctions[c.Func.Name]; ok {
//		return buildPipeline(
//			args[0],
//			call("map", map[string]influxql.Expression{"fn": singleArgFloatFn(fn, "x")}),
//			dropFieldAndTimeCall,
//		), nil
//	}
//
//	// day_of_month(), hour(), etc.
//	if fn, ok := dateFunctions[c.Func.Name]; ok {
//		var v influxql.Expression
//		if len(args) == 0 {
//			v = t.timeFn()
//		} else {
//			v = args[0]
//		}
//
//		return buildPipeline(
//			v,
//			call("map", map[string]influxql.Expression{"fn": singleArgFloatFn(fn, "timestamp")}),
//			dropFieldAndTimeCall,
//		), nil
//	}
//
//	switch c.Func.Name {
//	case "rate", "delta", "increase":
//		isCounter := true
//		isRate := true
//
//		if c.Func.Name == "delta" {
//			isCounter = false
//			isRate = false
//		}
//		if c.Func.Name == "increase" {
//			isRate = false
//		}
//
//		return buildPipeline(
//			args[0],
//			call("parser.extrapolatedRate", map[string]influxql.Expression{
//				"isCounter": &influxql.BooleanLiteral{Value: isCounter},
//				"isRate":    &influxql.BooleanLiteral{Value: isRate},
//			}),
//			dropFieldAndTimeCall,
//		), nil
//	case "irate", "idelta":
//		isRate := true
//
//		if c.Func.Name == "idelta" {
//			isRate = false
//		}
//
//		return buildPipeline(
//			args[0],
//			call("parser.instantRate", map[string]influxql.Expression{
//				"isRate": &influxql.BooleanLiteral{Value: isRate},
//			}),
//			dropFieldAndTimeCall,
//		), nil
//	case "deriv":
//		return buildPipeline(
//			args[0],
//			call("parser.linearRegression", nil),
//			dropFieldAndTimeCall,
//		), nil
//	case "predict_linear":
//		if yieldsTable(c.Args[1]) {
//			return nil, errors.Errorf("non-const scalar expressions not supported yet")
//		}
//
//		return buildPipeline(
//			args[0],
//			call("parser.linearRegression", map[string]influxql.Expression{
//				"predict": &influxql.BooleanLiteral{Value: true},
//				"fromNow": args[1],
//			}),
//			dropFieldAndTimeCall,
//		), nil
//	case "holt_winters":
//		if yieldsTable(c.Args[1]) || yieldsTable(c.Args[2]) {
//			return nil, errors.Errorf("non-const scalar expressions not supported yet")
//		}
//
//		return buildPipeline(
//			args[0],
//			call("parser.holtWinters", map[string]influxql.Expression{
//				"smoothingFactor": args[1],
//				"trendFactor":     args[2],
//			}),
//			dropFieldAndTimeCall,
//		), nil
//	case "timestamp":
//		return buildPipeline(
//			args[0],
//			call("parser.timestamp", nil),
//			dropFieldAndTimeCall,
//		), nil
//	case "time":
//		return t.timeFn(), nil
//	case "changes", "resets":
//		fn := "parser." + c.Func.Name
//
//		return buildPipeline(
//			args[0],
//			call(fn, nil),
//			dropFieldAndTimeCall,
//		), nil
//	case "clamp_max", "clamp_min":
//		fn := "mMax"
//		if c.Func.Name == "clamp_max" {
//			fn = "mMin"
//		}
//
//		v := args[0]
//		clamp := args[1]
//		return buildPipeline(
//			v,
//			call("map", map[string]influxql.Expression{
//				"fn": scalarArithBinaryMathFn(fn, clamp, false),
//			}),
//			dropFieldAndTimeCall,
//		), nil
//	case "label_join":
//		v := args[0]
//
//		dst, ok := args[1].(*influxql.StringLiteral)
//		if !ok {
//			return nil, errors.Errorf("label_join() destination label must be string literal")
//		}
//		if !model.LabelName(dst.Value).IsValid() {
//			return nil, errors.Errorf("invalid destination label name in label_join(): %s", dst.Value)
//		}
//		dst.Value = escapeLabelName(dst.Value)
//
//		sep, ok := args[2].(*influxql.StringLiteral)
//		if !ok {
//			return nil, errors.Errorf("label_join() separator must be string literal")
//		}
//
//		srcLabels := make([]*influxql.StringLiteral, len(args)-3)
//		for i := 3; i < len(args); i++ {
//			src, ok := args[i].(*influxql.StringLiteral)
//			if !ok {
//				return nil, errors.Errorf("label_join() source labels must be string literals")
//			}
//			if !model.LabelName(src.Value).IsValid() {
//				return nil, errors.Errorf("invalid source label name in label_join(): %s", src.Value)
//			}
//			src.Value = escapeLabelName(src.Value)
//			srcLabels[i-3] = src
//		}
//
//		return buildPipeline(
//			v,
//			call("map", map[string]influxql.Expression{"fn": labelJoinFn(srcLabels, dst, sep)}),
//		), nil
//	case "label_replace":
//		for _, arg := range args[1:] {
//			if _, ok := arg.(*influxql.StringLiteral); !ok {
//				return nil, errors.Errorf("non-literal string arguments not supported yet in label_replace()")
//			}
//		}
//
//		dst, ok := args[1].(*influxql.StringLiteral)
//		if !ok {
//			return nil, errors.Errorf("label_replace() destination label must be string literal")
//		}
//		if !model.LabelName(dst.Value).IsValid() {
//			return nil, errors.Errorf("invalid destination label name in label_replace(): %s", dst.Value)
//		}
//		dst.Value = escapeLabelName(dst.Value)
//
//		repl, ok := args[2].(*influxql.StringLiteral)
//		if !ok {
//			return nil, errors.Errorf("label_replace() destination label must be string literal")
//		}
//
//		src, ok := args[3].(*influxql.StringLiteral)
//		if !ok {
//			return nil, errors.Errorf("label_replace() source label must be string literal")
//		}
//		// We explicitly do *not* check the validity of the source label here, as PromQL's label_replace()
//		// also allows invalid source labels.
//		src.Value = escapeLabelName(src.Value)
//
//		regex, ok := args[4].(*influxql.StringLiteral)
//		if !ok {
//			return nil, errors.Errorf("label_replace() source label must be string literal")
//		}
//
//		return buildPipeline(
//			args[0],
//			call("parser.labelReplace", map[string]influxql.Expression{
//				"destination": dst,
//				"replacement": repl,
//				"source":      src,
//				"regex":       regex,
//			}),
//		), nil
//	case "vector":
//		if yieldsTable(c.Args[0]) {
//			return args[0], nil
//		}
//		return buildPipeline(
//			t.generateZeroWindows(),
//			call("map", map[string]influxql.Expression{
//				"fn": setConstValueFn(args[0]),
//			}),
//		), nil
//	case "scalar":
//		// TODO: Need to insert NaN values at time steps where there is no value in the vector.
//		// This requires new outer join support.
//		return buildPipeline(
//			args[0],
//			call("keep", map[string]influxql.Expression{
//				"columns": columnList("_stop", "_value"),
//			}),
//		), nil
//	case "histogram_quantile":
//		if yieldsTable(c.Args[0]) {
//			return nil, errors.Errorf("non-const scalar expressions not supported yet")
//		}
//
//		return buildPipeline(
//			args[1],
//			call("group", map[string]influxql.Expression{
//				"columns": columnList("_time", "_value", "le"),
//				"mode":    &influxql.StringLiteral{Value: "except"},
//			}),
//			call("parser.promHistogramQuantile", map[string]influxql.Expression{
//				"quantile": args[0],
//			}),
//			dropFieldAndTimeCall,
//		), nil
//	default:
//		return nil, errors.Errorf("PromQL function %q is not supported yet", c.Func.Name)
//	}
//}
