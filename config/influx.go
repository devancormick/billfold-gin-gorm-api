package config

import (
	"context"
	"os"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
)

var influxWrite api.WriteAPI

// ConnectInflux sets up the async InfluxDB write client used for
// request-latency and transaction-throughput metrics.
func ConnectInflux() {
	client := influxdb2.NewClient(os.Getenv("INFLUX_URL"), os.Getenv("INFLUX_TOKEN"))
	influxWrite = client.WriteAPI(os.Getenv("INFLUX_ORG"), os.Getenv("INFLUX_BUCKET"))
}

// RecordRequestLatency writes one latency sample per handled request.
// Non-blocking: InfluxDB write errors must never affect the API response.
func RecordRequestLatency(route string, status int, dur time.Duration) {
	if influxWrite == nil {
		return
	}
	p := influxdb2.NewPoint(
		"http_request",
		map[string]string{"route": route},
		map[string]interface{}{
			"status_code": status,
			"duration_ms": dur.Milliseconds(),
		},
		time.Now(),
	)
	influxWrite.WritePoint(p)
}

// FlushInflux blocks until buffered points are written; call on shutdown.
func FlushInflux(ctx context.Context) {
	if influxWrite == nil {
		return
	}
	influxWrite.Flush()
}
