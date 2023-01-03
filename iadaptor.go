package promql2influxql

import "github.com/wubin1989/promql2influxql/command"

type IAdaptor interface {
	Query(c command.Command) (command.CommandResult, error)
	Write(c command.Command) error
}
