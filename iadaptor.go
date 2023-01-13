package promql2influxql

import (
	"context"
	"github.com/wubin1989/promql2influxql/command"
)

// IAdaptor is an interface for concrete struct to implementing. There is only one method Query currently.
// It accepts a command.Command which wraps all TSDB query related parameters
// and return an interface{} for accepting any kind of result.
type IAdaptor interface {
	Query(ctx context.Context, cmd command.Command) (interface{}, error)
}
