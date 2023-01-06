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

func (t *Transpiler) transpileVectorSelector2ConditionExpr(v *parser.VectorSelector) (influxql.Expr, error) {
	now := time.Now()
	var start, end *time.Time

	end = &now
	if t.Evaluation != nil {
		end = t.Evaluation
	}
	if t.Start != nil || t.End != nil {
		if t.Start != nil {
			start = t.Start
		}
		if t.End != nil {
			end = t.End
		}
	} else if v.Timestamp != nil {
		ts := time.UnixMilli(*v.Timestamp)
		end = &ts
	}
	if start == nil {
		ts := end.Add(-v.OriginalOffset)
		end = &ts
	}

	binaryExpr := &influxql.BinaryExpr{
		Op: influxql.LT,
		LHS: &influxql.VarRef{
			Val: "time",
		},
		RHS: &influxql.TimeLiteral{
			Val: *end,
		},
	}

	if start != nil || len(v.LabelMatchers) > 0 {
		condition := (*Condition)(binaryExpr)
		if start != nil {
			condition = condition.And(&influxql.BinaryExpr{
				Op: influxql.GTE,
				LHS: &influxql.VarRef{
					Val: "time",
				},
				RHS: &influxql.TimeLiteral{
					Val: *start,
				},
			})
		}
		if len(v.LabelMatchers) > 0 {
			for _, item := range v.LabelMatchers {
				if _, ok := reservedTags[item.Name]; ok {
					continue
				}
				switch item.Type {
				case labels.MatchEqual:
					condition = condition.And(&influxql.BinaryExpr{
						Op: influxql.EQ,
						LHS: &influxql.VarRef{
							Val: item.Name,
						},
						RHS: &influxql.StringLiteral{
							Val: item.Value,
						},
					})
				case labels.MatchNotEqual:
					condition = condition.And(&influxql.BinaryExpr{
						Op: influxql.NEQ,
						LHS: &influxql.VarRef{
							Val: item.Name,
						},
						RHS: &influxql.StringLiteral{
							Val: item.Value,
						},
					})
				case labels.MatchRegexp, labels.MatchNotRegexp:
					promRegexStr := "^(?:" + item.Value + ")$"
					re, err := regexp.Compile(promRegexStr)
					if err != nil {
						return nil, errors.Wrap(err, "regular expression syntax error")
					}
					cond := &influxql.BinaryExpr{
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
					condition = condition.And(cond)
				default:
					return nil, errors.Errorf("not support PromQL match type %s", item.Type)
				}
			}
		}
		binaryExpr = (*influxql.BinaryExpr)(condition)
	}

	return binaryExpr, nil
}

func (t *Transpiler) transpileInstantVectorSelector(v *parser.VectorSelector) (influxql.Node, error) {
	condition, err := t.transpileVectorSelector2ConditionExpr(v)
	if err != nil {
		return nil, errors.Wrap(err, "transpile instant vector selector fail")
	}
	selectStatement := influxql.SelectStatement{
		Fields: []*influxql.Field{
			{Expr: &influxql.Wildcard{}},
		},
		Sources:    []influxql.Source{&influxql.Measurement{Name: v.Name}},
		Condition:  condition,
		Location:   t.Timezone,
		Dimensions: []*influxql.Dimension{{Expr: &influxql.Wildcard{}}},
	}
	if t.Start == nil && t.End == nil {
		selectStatement.Limit = 1
	}
	return &selectStatement, nil
}

func (t *Transpiler) transpileRangeVectorSelector(v *parser.MatrixSelector) (influxql.Node, error) {
	if t.Start != nil {
		return t.transpileInstantVectorSelector(v.VectorSelector.(*parser.VectorSelector))
	}
	now := time.Now()
	end := &now
	if t.Evaluation != nil {
		end = t.Evaluation
	}
	if t.End != nil {
		end = t.End
	}
	start := end.Add(-v.Range)
	transpiler := NewTranspiler(&start, end, WithTimezone(t.Timezone))
	return transpiler.transpileInstantVectorSelector(v.VectorSelector.(*parser.VectorSelector))
}
