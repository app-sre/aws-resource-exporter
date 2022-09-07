package pkg

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func createTestMetric(fqdn string, value float64) prometheus.Metric {
	desc := prometheus.NewDesc(fqdn, "help", []string{"labels"}, nil)
	return prometheus.MustNewConstMetric(desc, prometheus.CounterValue, 1, "test")
}

func TestGetMetricHash(t *testing.T) {
	assert.Equal(t, "3dd70cc3f27fa55012a996a0a36412073f2ced7cc3b5fc93933f2c79f8c8aaf5", getMetricHash(createTestMetric("foo_bar", 1)))
	assert.Equal(t, "3dd70cc3f27fa55012a996a0a36412073f2ced7cc3b5fc93933f2c79f8c8aaf5", getMetricHash(createTestMetric("foo_bar", 10)))
	assert.NotEqual(t, "3dd70cc3f27fa55012a996a0a36412073f2ced7cc3b5fc93933f2c79f8c8aaf5", getMetricHash(createTestMetric("other", 1)))
}

func TestMetricCacheGetAllWithTTL(t *testing.T) {
	cache := NewMetricsCache(1)

	testMetric := createTestMetric("testing", 1)
	cache.AddMetric(testMetric)
	assert.Len(t, cache.entries, 1)

	assert.Equal(t, []prometheus.Metric{testMetric}, cache.GetAllMetrics())
	time.Sleep(2 * time.Second)
	assert.Len(t, cache.GetAllMetrics(), 0)
}
