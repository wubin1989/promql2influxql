package command

import "time"

const LineBreak = "\n"

// OperationType indicates a Command is for writing or for querying
type OperationType int

const (
	QUERY_OPERATION OperationType = iota + 1
	WRITE_OPERATION               = iota + 1
)

// DialectType is alias of string type and indicates which query grammar a Command is using
type DialectType string

// CommandType indicates the type of Command in business meaning
type CommandType struct {
	OperationType OperationType
	DialectType   DialectType
}

// DataType indicates a Command is for table display or for graph display in data visualization platform like Grafana
// Basically,
//  - TABLE_DATA is for raw table data query
//  - GRAPH_DATA is for time-bounding graph data query
// 	- LABEL_VALUES_DATA is for label values data query like fetching data for label selector options at the top of Grafana dashboard
type DataType int

const (
	TABLE_DATA DataType = iota + 1
	GRAPH_DATA
	LABEL_VALUES_DATA
)

// Command wraps a raw query expression with several related attributes
type Command struct {
	// Cmd is a raw query expression
	Cmd     string
	Dialect DialectType

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
