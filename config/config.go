package config

import "time"

const (
	DefaultQueryTimeout = "2m"
)

// Config configures promql2influxql.InfluxDBAdaptor
type Config struct {
	// Timeout sets timeout duration for a single query execution
	Timeout time.Duration
	// Verbose indicates whether to output more logs or not
	Verbose bool
}

func NewConfig() Config {
	timeout, _ := time.ParseDuration(DefaultQueryTimeout)
	cfg := Config{
		Timeout: timeout,
	}
	return cfg
}
