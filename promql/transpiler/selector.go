package transpiler

import (
	"github.com/influxdata/influxql"
	"github.com/pkg/errors"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/promql/parser"
	"regexp"
	"time"
)

var reservedTags = map[string]struct{}{
	"__name__": {},
}

var labelMatchOps = map[labels.MatchType]influxql.Token{
	labels.MatchEqual:     influxql.EQ,
	labels.MatchNotEqual:  influxql.NEQ,
	labels.MatchRegexp:    influxql.EQREGEX,
	labels.MatchNotRegexp: influxql.NEQREGEX,
}

func (t *Transpiler) transpileVectorSelector2ConditionExpr(v *parser.VectorSelector) (timeCondition influxql.Expr, tagCondition influxql.Expr, err error) {
	now := time.Now()
	var start, end *time.Time
	end = &now
	if t.Evaluation != nil {
		end = t.Evaluation
	}
	if v.Timestamp != nil {
		ts := time.UnixMilli(*v.Timestamp)
		end = &ts
	}
	if t.End != nil {
		end = t.End
	}
	endTs := end.Add(-v.OriginalOffset)
	end = &endTs

	switch t.DataType {
	case GRAPH_DATA:
		if t.Start != nil {
			start = t.Start
		}
	default:
		if t.timeRange > 0 {
			startTs := end.Add(-t.timeRange)
			start = &startTs
		}
	}

	timeBinExpr := &influxql.BinaryExpr{
		Op: influxql.LTE,
		LHS: &influxql.VarRef{
			Val: "time",
		},
		RHS: &influxql.TimeLiteral{
			Val: *end,
		},
	}
	if start != nil {
		timeCond := (*Condition)(timeBinExpr)
		timeCond = timeCond.And(&influxql.BinaryExpr{
			Op: influxql.GTE,
			LHS: &influxql.VarRef{
				Val: "time",
			},
			RHS: &influxql.TimeLiteral{
				Val: *start,
			},
		})
		timeCondition = (*influxql.BinaryExpr)(timeCond)
	} else {
		timeCondition = timeBinExpr
	}

	var tagCond *Condition
	for _, item := range v.LabelMatchers {
		if _, ok := reservedTags[item.Name]; ok {
			continue
		}
		var cond *influxql.BinaryExpr
		switch item.Type {
		case labels.MatchEqual:
			cond = &influxql.BinaryExpr{
				Op: influxql.EQ,
				LHS: &influxql.VarRef{
					Val: item.Name,
				},
				RHS: &influxql.StringLiteral{
					Val: item.Value,
				},
			}
		case labels.MatchNotEqual:
			cond = &influxql.BinaryExpr{
				Op: influxql.NEQ,
				LHS: &influxql.VarRef{
					Val: item.Name,
				},
				RHS: &influxql.StringLiteral{
					Val: item.Value,
				},
			}
		case labels.MatchRegexp, labels.MatchNotRegexp:
			promRegexStr := "^(?:" + item.Value + ")$"
			re, err := regexp.Compile(promRegexStr)
			if err != nil {
				return nil, nil, errors.Wrap(err, "regular expression syntax error")
			}
			cond = &influxql.BinaryExpr{
				Op: influxql.EQREGEX,
				LHS: &influxql.VarRef{
					Val: item.Name,
				},
				RHS: &influxql.RegexLiteral{
					Val: re,
				},
			}
			if item.Type == labels.MatchNotRegexp {
				cond.Op = influxql.NEQREGEX
			}
		default:
			return nil, nil, errors.Errorf("not support PromQL match type %s", item.Type)
		}
		if tagCond != nil {
			tagCond = tagCond.Add(cond)
		} else {
			tagCond = (*Condition)(cond)
		}
	}

	if tagCond != nil {
		tagCondition = (*influxql.BinaryExpr)(tagCond)
	}

	return
}

func (t *Transpiler) transpileInstantVectorSelector(v *parser.VectorSelector) (influxql.Node, error) {
	var (
		err          error
		tagCondition influxql.Expr
	)
	t.timeCondition, tagCondition, err = t.transpileVectorSelector2ConditionExpr(v)
	if err != nil {
		return nil, errors.Wrap(err, "transpile instant vector selector fail")
	}
	selectStatement := influxql.SelectStatement{
		Fields: []*influxql.Field{
			{
				Expr: &influxql.Wildcard{
					Type: influxql.TAG,
				},
			},
		},
		Condition:  tagCondition,
		Sources:    []influxql.Source{&influxql.Measurement{Name: v.Name}},
		Dimensions: []*influxql.Dimension{{Expr: &influxql.Wildcard{}}},
	}
	if t.timeRange > 0 {
		selectStatement.Fields = append(selectStatement.Fields, &influxql.Field{
			Expr: &influxql.VarRef{
				Val: "value",
			},
		})
	} else {
		selectStatement.Fields = append(selectStatement.Fields, &influxql.Field{
			Expr: &influxql.Call{
				Name: "last",
				Args: []influxql.Expr{
					&influxql.VarRef{
						Val: "value",
					},
				},
			},
		})
	}
	return &selectStatement, nil
}

func (t *Transpiler) transpileRangeVectorSelector(v *parser.MatrixSelector) (influxql.Node, error) {
	if v.Range > 0 {
		t.timeRange = v.Range
	}
	return t.transpileExpr(v.VectorSelector)
}
