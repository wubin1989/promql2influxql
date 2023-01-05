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

type Command struct {
	Cmd     string      `json:"cmd"`
	Dialect DialectType `json:"dialect"`

	Database string `json:"database"`
	// Start and End attributes are used for PromQL as it doesn't support time range itself
	Start *time.Time `json:"start"`
	End   *time.Time `json:"end"`
}
