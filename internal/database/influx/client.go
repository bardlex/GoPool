// Package influx provides InfluxDB client and time-series data operations for GOMP mining pool.
// It handles metrics collection, hashrate tracking, and statistical data storage.
package influx

import (
	"context"
	"fmt"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
)

// Client wraps InfluxDB operations for time-series metrics
type Client struct {
	client   influxdb2.Client
	writeAPI api.WriteAPI
	queryAPI api.QueryAPI
	bucket   string
	org      string
}

// Config holds InfluxDB connection configuration
type Config struct {
	URL    string
	Token  string
	Org    string
	Bucket string
}

// NewClient creates a new InfluxDB client
func NewClient(cfg *Config) (*Client, error) {
	client := influxdb2.NewClient(cfg.URL, cfg.Token)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	health, err := client.Health(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check InfluxDB health: %w", err)
	}

	if health.Status != "pass" {
		msg := ""
		if health.Message != nil {
			msg = *health.Message
		}
		return nil, fmt.Errorf("InfluxDB health check failed: %s", msg)
	}

	writeAPI := client.WriteAPI(cfg.Org, cfg.Bucket)
	queryAPI := client.QueryAPI(cfg.Org)

	return &Client{
		client:   client,
		writeAPI: writeAPI,
		queryAPI: queryAPI,
		bucket:   cfg.Bucket,
		org:      cfg.Org,
	}, nil
}

// Close closes the InfluxDB connection
func (c *Client) Close() {
	c.writeAPI.Flush()
	c.client.Close()
}

// Health checks InfluxDB connectivity
func (c *Client) Health(ctx context.Context) error {
	health, err := c.client.Health(ctx)
	if err != nil {
		return fmt.Errorf("failed to check health: %w", err)
	}

	if health.Status != "pass" {
		msg := ""
		if health.Message != nil {
			msg = *health.Message
		}
		return fmt.Errorf("health check failed: %s", msg)
	}

	return nil
}

// Mining metrics

// WriteShareMetric writes a share submission metric
func (c *Client) WriteShareMetric(userID, workerID int64, difficulty, networkDiff float64, isValid, isBlock bool) {
	tags := map[string]string{
		"user_id":   fmt.Sprintf("%d", userID),
		"worker_id": fmt.Sprintf("%d", workerID),
		"valid":     fmt.Sprintf("%t", isValid),
		"block":     fmt.Sprintf("%t", isBlock),
	}

	fields := map[string]interface{}{
		"difficulty":         difficulty,
		"network_difficulty": networkDiff,
		"count":              1,
	}

	point := write.NewPoint("shares", tags, fields, time.Now())
	c.writeAPI.WritePoint(point)
}

// WriteHashrateMetric writes a hashrate measurement
func (c *Client) WriteHashrateMetric(userID, workerID int64, hashrate float64) {
	tags := map[string]string{
		"user_id":   fmt.Sprintf("%d", userID),
		"worker_id": fmt.Sprintf("%d", workerID),
	}

	fields := map[string]interface{}{
		"hashrate": hashrate,
	}

	point := write.NewPoint("hashrate", tags, fields, time.Now())
	c.writeAPI.WritePoint(point)
}

// WriteBlockMetric writes a block discovery metric
func (c *Client) WriteBlockMetric(height int64, hash string, userID, workerID *int64, difficulty, reward float64, status string) {
	tags := map[string]string{
		"status": status,
		"hash":   hash,
	}

	if userID != nil {
		tags["user_id"] = fmt.Sprintf("%d", *userID)
	}
	if workerID != nil {
		tags["worker_id"] = fmt.Sprintf("%d", *workerID)
	}

	fields := map[string]interface{}{
		"height":     height,
		"difficulty": difficulty,
		"reward":     reward,
		"count":      1,
	}

	point := write.NewPoint("blocks", tags, fields, time.Now())
	c.writeAPI.WritePoint(point)
}

// WritePayoutMetric writes a payout metric
func (c *Client) WritePayoutMetric(userID int64, amount float64, status string) {
	tags := map[string]string{
		"user_id": fmt.Sprintf("%d", userID),
		"status":  status,
	}

	fields := map[string]interface{}{
		"amount": amount,
		"count":  1,
	}

	point := write.NewPoint("payouts", tags, fields, time.Now())
	c.writeAPI.WritePoint(point)
}

// Pool statistics

// WritePoolStatsMetric writes overall pool statistics
func (c *Client) WritePoolStatsMetric(totalHashrate float64, activeWorkers, activeUsers, totalShares, validShares, invalidShares int64, networkHashrate, networkDiff float64) {
	fields := map[string]interface{}{
		"total_hashrate":     totalHashrate,
		"active_workers":     activeWorkers,
		"active_users":       activeUsers,
		"total_shares":       totalShares,
		"valid_shares":       validShares,
		"invalid_shares":     invalidShares,
		"network_hashrate":   networkHashrate,
		"network_difficulty": networkDiff,
	}

	point := write.NewPoint("pool_stats", map[string]string{}, fields, time.Now())
	c.writeAPI.WritePoint(point)
}

// WriteConnectionMetric writes connection statistics
func (c *Client) WriteConnectionMetric(activeConnections, totalConnections int64, avgLatency float64) {
	fields := map[string]interface{}{
		"active_connections": activeConnections,
		"total_connections":  totalConnections,
		"avg_latency":        avgLatency,
	}

	point := write.NewPoint("connections", map[string]string{}, fields, time.Now())
	c.writeAPI.WritePoint(point)
}

// System metrics

// WriteSystemMetric writes system performance metrics
func (c *Client) WriteSystemMetric(service string, cpuUsage, memoryUsage float64, goroutines int64) {
	tags := map[string]string{
		"service": service,
	}

	fields := map[string]interface{}{
		"cpu_usage":    cpuUsage,
		"memory_usage": memoryUsage,
		"goroutines":   goroutines,
	}

	point := write.NewPoint("system", tags, fields, time.Now())
	c.writeAPI.WritePoint(point)
}

// Query methods

// GetHashrateHistory retrieves hashrate history for a user/worker
func (c *Client) GetHashrateHistory(ctx context.Context, userID, workerID int64, duration time.Duration) ([]HashratePoint, error) {
	query := fmt.Sprintf(`
		from(bucket: "%s")
		|> range(start: -%s)
		|> filter(fn: (r) => r._measurement == "hashrate")
		|> filter(fn: (r) => r.user_id == "%d")
		|> filter(fn: (r) => r.worker_id == "%d")
		|> filter(fn: (r) => r._field == "hashrate")
		|> aggregateWindow(every: 5m, fn: mean, createEmpty: false)
	`, c.bucket, duration.String(), userID, workerID)

	result, err := c.queryAPI.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query hashrate history: %w", err)
	}
	defer func() {
		if err := result.Close(); err != nil {
			// Note: Consider adding structured logging here in the future
			_ = err // Ignore close errors for now
		}
	}()

	var points []HashratePoint
	for result.Next() {
		record := result.Record()
		if value, ok := record.Value().(float64); ok {
			points = append(points, HashratePoint{
				Time:     record.Time(),
				Hashrate: value,
			})
		}
	}

	if result.Err() != nil {
		return nil, fmt.Errorf("error reading query result: %w", result.Err())
	}

	return points, nil
}

// GetShareStats retrieves share statistics for a time period
func (c *Client) GetShareStats(ctx context.Context, userID int64, duration time.Duration) (*ShareStats, error) {
	query := fmt.Sprintf(`
		from(bucket: "%s")
		|> range(start: -%s)
		|> filter(fn: (r) => r._measurement == "shares")
		|> filter(fn: (r) => r.user_id == "%d")
		|> filter(fn: (r) => r._field == "count")
		|> group(columns: ["valid"])
		|> sum()
	`, c.bucket, duration.String(), userID)

	result, err := c.queryAPI.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query share stats: %w", err)
	}
	defer func() {
		if err := result.Close(); err != nil {
			// Note: Consider adding structured logging here in the future
			_ = err // Ignore close errors for now
		}
	}()

	stats := &ShareStats{}
	for result.Next() {
		record := result.Record()
		if count, ok := record.Value().(int64); ok {
			if record.ValueByKey("valid") == "true" {
				stats.ValidShares = count
			} else {
				stats.InvalidShares = count
			}
		}
	}

	if result.Err() != nil {
		return nil, fmt.Errorf("error reading query result: %w", result.Err())
	}

	stats.TotalShares = stats.ValidShares + stats.InvalidShares
	if stats.TotalShares > 0 {
		stats.ValidPercent = float64(stats.ValidShares) / float64(stats.TotalShares) * 100
	}

	return stats, nil
}

// GetPoolHashrate retrieves current pool hashrate
func (c *Client) GetPoolHashrate(ctx context.Context, duration time.Duration) (float64, error) {
	query := fmt.Sprintf(`
		from(bucket: "%s")
		|> range(start: -%s)
		|> filter(fn: (r) => r._measurement == "hashrate")
		|> filter(fn: (r) => r._field == "hashrate")
		|> aggregateWindow(every: 5m, fn: mean, createEmpty: false)
		|> group()
		|> sum()
		|> last()
	`, c.bucket, duration.String())

	result, err := c.queryAPI.Query(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to query pool hashrate: %w", err)
	}
	defer func() {
		if err := result.Close(); err != nil {
			// Note: Consider adding structured logging here in the future
			_ = err // Ignore close errors for now
		}
	}()

	if result.Next() {
		record := result.Record()
		if hashrate, ok := record.Value().(float64); ok {
			return hashrate, nil
		}
	}

	if result.Err() != nil {
		return 0, fmt.Errorf("error reading query result: %w", result.Err())
	}

	return 0, nil
}

// Flush forces a write of all pending points
func (c *Client) Flush() {
	c.writeAPI.Flush()
}

// Data structures

// HashratePoint represents a hashrate measurement at a point in time
type HashratePoint struct {
	Time     time.Time `json:"time"`
	Hashrate float64   `json:"hashrate"`
}

// ShareStats represents aggregated share statistics
type ShareStats struct {
	TotalShares   int64   `json:"total_shares"`
	ValidShares   int64   `json:"valid_shares"`
	InvalidShares int64   `json:"invalid_shares"`
	ValidPercent  float64 `json:"valid_percent"`
}
