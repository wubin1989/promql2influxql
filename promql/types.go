package promql

import (
	"fmt"
	"github.com/prometheus/prometheus/promql/parser"
)

type promAPIFuncResult struct {
	data interface{}
	err  *promAPIError
}

type promErrorType string

const (
	promErrorNone        promErrorType = ""
	promErrorTimeout     promErrorType = "timeout"
	promErrorCanceled    promErrorType = "canceled"
	promErrorExec        promErrorType = "execution"
	promErrorBadData     promErrorType = "bad_data"
	promErrorInternal    promErrorType = "internal"
	promErrorUnavailable promErrorType = "unavailable"
	promErrorNotFound    promErrorType = "not_found"
)

type promAPIError struct {
	typ promErrorType
	err error
}

func (e *promAPIError) Error() string {
	return fmt.Sprintf("%s: %s", e.typ, e.err)
}

type promQueryData struct {
	ResultType parser.ValueType `json:"resultType"`
	Result     parser.Value     `json:"result"`
}
