package pkg

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
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
	var dto dto.Metric
	metric.Write(&dto)
	labelString := metric.Desc().String()

	for _, labelPair := range dto.GetLabel() {
		labelString = fmt.Sprintf("%s,%s,%s", labelString, labelPair.GetName(), labelPair.GetValue())
	}

	checksum := sha256.Sum256([]byte(labelString))
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
