package service

import (
	"context"
	"github.com/wubin1989/promql2influxql/applications/prom/dto"
)

//go:generate go-doudou svc http -c
//go:generate go-doudou svc grpc

type Prom interface {
	// Query is compatible to Prometheus POST /api/v1/query
	Query(ctx context.Context,
	// Prometheus expression query string.
	//
	// Example: "?query=up"
	//
	// required
		query string,
	// Evaluation timestamp. Optional.
	//
	// The current server time is used if the "time" parameter is omitted.
	//
	// Optional.
		time *string,
	// Evaluation timeout. Optional.
		timeout *string) (data dto.QueryData, status string, err error)

	// GetQuery is compatible to Prometheus GET /api/v1/query
	GetQuery(ctx context.Context,
	// Prometheus expression query string.
	//
	// Example: "?query=up"
	//
	// required
		query string,
	// Evaluation timestamp. Optional.
	//
	// The current server time is used if the "time" parameter is omitted.
	//
	// Optional.
		time *string,
	// Evaluation timeout. Optional.
		timeout *string) (data dto.QueryData, status string, err error)

	// Query_range is compatible to Prometheus POST /api/v1/query_range
	Query_range(ctx context.Context,
	// Prometheus expression query string.
	//
	// Example: "?query=up"
	//
	// required
		query string,
	// Start timestamp.
	//
	// Example: "&start=2015-07-01T20:10:30.781Z"
	//
		start *string,
	// End timestamp.
	//
	// Example: "&end=2015-07-01T20:11:00.781Z"
	//
		end *string,
	// Query resolution step width in "duration" format or float number of seconds.
	//
	// Example: "&step=15s"
	//
		step *string,
	// Evaluation timeout. Optional.
		timeout *string) (data dto.QueryData, status string, err error)

	// GetQuery_range is compatible to Prometheus GET /api/v1/query_range
	GetQuery_range(ctx context.Context,
	// Prometheus expression query string.
	//
	// Example: "?query=up"
	//
	// required
		query string,
	// Start timestamp.
	//
	// Example: "&start=2015-07-01T20:10:30.781Z"
	//
		start *string,
	// End timestamp.
	//
	// Example: "&end=2015-07-01T20:11:00.781Z"
	//
		end *string,
	// Query resolution step width in "duration" format or float number of seconds.
	//
	// Example: "&step=15s"
	//
		step *string,
	// Evaluation timeout. Optional.
		timeout *string) (data dto.QueryData, status string, err error)

	// GetLabel_Label_nameValues Returns label values
	// The following endpoint returns a list of label values for a provided label name
	//
	// The "data" section of the JSON response is a list of string label values.
	//
	GetLabel_Label_nameValues(ctx context.Context,
	// Start timestamp. Optional.
	//
		start *string,
	// End timestamp. Optional.
	//
		end *string,
	// Repeated series selector argument that selects the series from which to read the label values. Optional.
	//
		match *[]string,
	// Label name
	//
	// Example: "/label/job/values"
	//
	// required
		label_name string) (data []string, status string, err error)
}
