package cache

import (
	"context"
	"time"
)

// CacheMetrics reports store-level cache counters.
//
// Evictions includes both policy-driven evictions and explicit removals such as
// invalidation, prefix invalidation, and flush operations.
type CacheMetrics struct {
	Hits      int64 `json:"hits"`
	Misses    int64 `json:"misses"`
	Evictions int64 `json:"evictions"`
	BytesUsed int64 `json:"bytes_used"`
}

// Metrics returns the current cache counters.
func (s *Store) Metrics() CacheMetrics {
	if s == nil {
		return CacheMetrics{}
	}

	bytesUsed := s.bytesUsed.Load()
	if bytesUsed < 0 {
		bytesUsed = 0
	}

	return CacheMetrics{
		Hits:      s.hits.Load(),
		Misses:    s.misses.Load(),
		Evictions: s.evictions.Load(),
		BytesUsed: bytesUsed,
	}
}

// StartMetricsLogger starts a background ticker that logs cache metric snapshots
// until ctx is cancelled.
func StartMetricsLogger(ctx context.Context, interval time.Duration, snapshot func() CacheMetrics, logf func(string, ...any)) {
	if interval <= 0 || snapshot == nil || logf == nil {
		return
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				metrics := snapshot()
				logf(
					"mcp cache metrics: hits=%d misses=%d evictions=%d bytes_used=%d",
					metrics.Hits,
					metrics.Misses,
					metrics.Evictions,
					metrics.BytesUsed,
				)
			}
		}
	}()
}
