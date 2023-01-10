package config

import "time"

const (
	DefaultQueryTimeout = "2m"
)

type Config struct {
	Timeout time.Duration
	Verbose bool
}

func NewConfig() Config {
	timeout, _ := time.ParseDuration(DefaultQueryTimeout)
	cfg := Config{
		Timeout: timeout,
	}
	return cfg
}
