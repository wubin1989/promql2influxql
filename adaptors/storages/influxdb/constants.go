package influxdb

type FunctionType int

const (
	AGGREGATE_FN FunctionType = iota + 1
	SELECTOR_FN
	TRANSFORM_FN
	PREDICTOR_FN
)
