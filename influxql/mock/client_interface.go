package mock

import (
	client "github.com/influxdata/influxdb1-client/v2"
	"time"
)

//go:generate mockgen -destination ./mock_client.go -package mock -source=./client_interface.go

// Client is a client interface for writing & querying the database.
type Client interface {
	// Ping checks that status of cluster, and will always return 0 time and no
	// error for UDP clients.
	Ping(timeout time.Duration) (time.Duration, string, error)

	// Write takes a BatchPoints object and writes all Points to InfluxDB.
	Write(bp client.BatchPoints) error

	// Query makes an InfluxDB Query on the database. This will fail if using
	// the UDP client.
	Query(q client.Query) (*client.Response, error)

	// QueryAsChunk makes an InfluxDB Query on the database. This will fail if using
	// the UDP client.
	QueryAsChunk(q client.Query) (*client.ChunkedResponse, error)

	// Close releases any resources a Client may be using.
	Close() error
}
