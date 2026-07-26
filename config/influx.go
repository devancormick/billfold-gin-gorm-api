package config

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// influxState holds the InfluxDB 3.x write target. InfluxDB 3's write API is
// a plain HTTP line-protocol endpoint (no official v2-style SDK client), so
// this talks to it directly rather than pulling in a client library.
var influxState struct {
	url   string // e.g. http://127.0.0.1:8181
	db    string
	token string
	queue chan string
	wg    sync.WaitGroup
}

// ConnectInflux starts a background writer that batches line-protocol points
// and flushes them to InfluxDB 3 on an interval. No-ops (queue stays nil) if
// INFLUX_URL is unset, so metrics writes are safe to call unconditionally.
func ConnectInflux() {
	url := os.Getenv("INFLUX_URL")
	if url == "" {
		return
	}

	influxState.url = strings.TrimSuffix(url, "/")
	influxState.db = os.Getenv("INFLUX_DB")
	influxState.token = os.Getenv("INFLUX_TOKEN")
	influxState.queue = make(chan string, 1000)

	influxState.wg.Add(1)
	go influxWriteLoop()

	log.Println("InfluxDB metrics writer started, db:", influxState.db)
}

func influxWriteLoop() {
	defer influxState.wg.Done()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var batch []string
	flush := func() {
		if len(batch) == 0 {
			return
		}
		writeBatch(batch)
		batch = batch[:0]
	}

	for {
		select {
		case line, ok := <-influxState.queue:
			if !ok {
				flush()
				return
			}
			batch = append(batch, line)
			if len(batch) >= 100 {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func writeBatch(lines []string) {
	body := strings.Join(lines, "\n")
	url := fmt.Sprintf("%s/api/v3/write_lp?db=%s&precision=millisecond", influxState.url, influxState.db)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(body))
	if err != nil {
		log.Println("influx: failed to build write request:", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+influxState.token)
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("influx: write failed:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		log.Println("influx: write rejected, status", resp.StatusCode)
	}
}

func escapeTagValue(v string) string {
	r := strings.NewReplacer(" ", "\\ ", ",", "\\,", "=", "\\=")
	return r.Replace(v)
}

// RecordRequestLatency enqueues one latency sample per handled request.
// Non-blocking: InfluxDB write errors must never affect the API response,
// and a full queue drops the point rather than stalling the request.
func RecordRequestLatency(route string, status int, dur time.Duration) {
	if influxState.queue == nil {
		return
	}
	if route == "" {
		route = "unmatched"
	}
	line := fmt.Sprintf(
		"http_request,route=%s status_code=%di,duration_ms=%di %d",
		escapeTagValue(route), status, dur.Milliseconds(), time.Now().UnixMilli(),
	)

	select {
	case influxState.queue <- line:
	default:
		// queue full — drop rather than block the request path
	}
}

// FlushInflux closes the write queue and waits for the final batch to flush.
func FlushInflux(ctx context.Context) {
	if influxState.queue == nil {
		return
	}
	close(influxState.queue)

	done := make(chan struct{})
	go func() {
		influxState.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
	}
}
