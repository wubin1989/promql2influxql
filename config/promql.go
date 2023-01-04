package config

type PromQLConfig struct {
	// EnableAtModifier if true enables @ modifier. Disabled otherwise. This
	// is supposed to be enabled for regular PromQL (as of Prometheus v2.33)
	// but the option to disable it is still provided here for those using
	// the Engine outside of Prometheus.
	EnableAtModifier bool

	// EnableNegativeOffset if true enables negative (-) offset
	// values. Disabled otherwise. This is supposed to be enabled for
	// regular PromQL (as of Prometheus v2.33) but the option to disable it
	// is still provided here for those using the Engine outside of
	// Prometheus.
	EnableNegativeOffset bool
}
