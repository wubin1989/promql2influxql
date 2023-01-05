package promql2influxql

import (
	"context"
	"github.com/wubin1989/promql2influxql/command"
)

type IAdaptor interface {
	Query(ctx context.Context, cmd command.Command) (interface{}, error)
}
