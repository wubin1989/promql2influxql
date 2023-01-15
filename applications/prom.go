package applications

import (
	"context"
	"time"
)

type IPromAdaptor interface {
	Query(ctx context.Context, cmd PromCommand) (RunResult, error)
}

// PromCommand wraps a raw query expression with several related attributes
type PromCommand struct {
	Cmd      string
	Database string
	// Start and End attributes are mainly used for PromQL currently
	// as it doesn't support time-bounding query expression itself
	Start      *time.Time
	End        *time.Time
	Timezone   *time.Location
	Evaluation *time.Time
	// Step is evaluation step for PromQL.
	// As InfluxQL doesn't have the equivalent expression or concept,
	// we use it as interval parameter for InfluxQL GROUP BY time(interval)
	// if the raw query doesn't contain PromQL MatrixSelector expression.
	// If the raw query does contain PromQL parser.MatrixSelector expression,
	// its Range attribute will be used as the interval parameter.
	Step time.Duration

	DataType DataType
	// ValueFieldKey indicates which field will be used.
	// As matrix value field as measurement in InfluxDB may contain multiple fields that is different from Prometheus,
	// so we may need to set ValueFieldKey.
	//
	// Default is ```value``` field.
	ValueFieldKey string
	// LabelName is only used for label values query.
	LabelName string
}

// RunResult wraps query result and possible error
type RunResult struct {
	Result     interface{}
	ResultType string
	Error      error
}
