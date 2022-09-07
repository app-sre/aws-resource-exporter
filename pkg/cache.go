package pkg

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type MetricsCache struct {
	cacheMutex   *sync.Mutex
	entries      map[string]CacheEntry
	ttlInSeconds int
}

func NewMetricsCache(ttlInSeconds int) *MetricsCache {
	return &MetricsCache{
		cacheMutex:   &sync.Mutex{},
		entries:      map[string]CacheEntry{},
		ttlInSeconds: ttlInSeconds,
	}
}

func getMetricHash(metric prometheus.Metric) string {
	checksum := sha256.Sum256([]byte(metric.Desc().String()))
	return fmt.Sprintf("%x", checksum[:])
}

func (mc *MetricsCache) AddMetric(metric prometheus.Metric) {
	mc.cacheMutex.Lock()
	mc.entries[getMetricHash(metric)] = CacheEntry{
		creation: time.Now(),
		metric:   metric,
	}
	mc.cacheMutex.Unlock()
}

func (mc *MetricsCache) GetAllMetrics() []prometheus.Metric {
	mc.cacheMutex.Lock()
	returnArr := make([]prometheus.Metric, 0)
	for k, v := range mc.entries {
		if time.Since(v.creation).Seconds() > float64(mc.ttlInSeconds) {
			delete(mc.entries, k)
		} else {
			returnArr = append(returnArr, v.metric)
		}
	}
	mc.cacheMutex.Unlock()
	return returnArr
}

type CacheEntry struct {
	creation time.Time
	metric   prometheus.Metric
}
