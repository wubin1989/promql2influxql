package command

import (
	"context"
	"github.com/prometheus/prometheus/promql/parser"
)

type StatementExecutor interface {
	// Exec processes the query. Can only be called once.
	Exec(ctx context.Context) (Result, error)
	Statement() parser.Statement
}
