package command

import (
	"context"
	"github.com/prometheus/prometheus/promql/parser"
)

type IStatementExecutor interface {
	// Exec processes the query. Can only be called once.
	Exec(ctx context.Context) (Result, error)
	Statement() parser.Statement
}

type ITranslator interface {
	Translate() (influxql string, ok bool)
}
