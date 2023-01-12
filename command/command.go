package command

import "time"

const LineBreak = "\n"

type OperationType int

const (
	QUERY_OPERATION OperationType = iota + 1
	WRITE_OPERATION               = iota + 1
)

type DialectType string

type CommandType struct {
	OperationType OperationType
	DialectType   DialectType
}

type DataType int

const (
	TABLE_DATA DataType = iota + 1
	GRAPH_DATA
	LABEL_VALUES_DATA
)

type Command struct {
	Cmd     string
	Dialect DialectType

	Database string
	// Start and End attributes are used for PromQL as it doesn't support time range itself
	Start      *time.Time
	End        *time.Time
	Timezone   *time.Location
	Evaluation *time.Time
	Step       time.Duration

	DataType      DataType
	ValueFieldKey string
	LabelName     string
}
