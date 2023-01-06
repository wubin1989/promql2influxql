package transpiler

import "github.com/pkg/errors"

var (
	ErrPromExprNotSupported = errors.New("not support PromQL expression")
)
