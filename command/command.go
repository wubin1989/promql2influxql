package command

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
}

// Row represents a single row returned from the execution of a statement.
type Row struct {
	Name    string            `json:"name,omitempty"`
	Tags    map[string]string `json:"tags,omitempty"`
	Columns []string          `json:"columns,omitempty"`
	Values  [][]interface{}   `json:"values,omitempty"`
}

// Result represents a resultset returned from a single statement.
type Result struct {
	Series []Row `json:"series"`
}

type CommandResult struct {
	Results []Result `json:"results"`
}
