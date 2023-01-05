package influxql

import "github.com/wubin1989/promql2influxql/command"

const (
	INFLUXQL_DIALECT command.DialectType = "influxql"
)

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

type QueryCommandResult struct {
	Results []Result `json:"results"`
}
