package config

import "time"

const (
	DefaultQueryMaxSamples = 50000000
	DefaultQueryTimeout    = "2m"
)

type Config struct {
	MaxSamples int
	Timeout    time.Duration
}

func NewConfig() Config {
	timeout, _ := time.ParseDuration(DefaultQueryTimeout)
	cfg := Config{
		MaxSamples: DefaultQueryMaxSamples,
		Timeout:    timeout,
	}
	return cfg
}
