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
	assert.Equal(t, "e2a622398e4a7ec5fca21510cc706fe28c1b37da32de620f0620804492890f62", getMetricHash(createTestMetric("foo_bar", 1)))
	assert.Equal(t, "e2a622398e4a7ec5fca21510cc706fe28c1b37da32de620f0620804492890f62", getMetricHash(createTestMetric("foo_bar", 10)))
	assert.NotEqual(t, "e2a622398e4a7ec5fca21510cc706fe28c1b37da32de620f0620804492890f62", getMetricHash(createTestMetric("other", 1)))
}

func TestSameMetricWithDifferentLabelsDontOverwrite(t *testing.T) {
	cache := NewMetricsCache(1 * time.Second)
	desc := prometheus.NewDesc("test", "multimetric", []string{"aws_region"}, nil)

	metricEast1 := prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, 1, "us-east-1")
	metricWest1 := prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, 1, "us-west-1")
	metricEast2 := prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, 2, "us-east-1")
	cache.AddMetric(metricEast1)
	cache.AddMetric(metricWest1) // should *not* overwrite metricEast1
	cache.AddMetric(metricEast2) // should overwrite metricEast1

	assert.Len(t, cache.GetAllMetrics(), 2)
	assert.NotContains(t, cache.GetAllMetrics(), metricEast1)
	assert.Contains(t, cache.GetAllMetrics(), metricWest1)
	assert.Contains(t, cache.GetAllMetrics(), metricEast2)
}

func TestMetricCacheGetAllWithTTL(t *testing.T) {
	cache := NewMetricsCache(1 * time.Second)

	testMetric := createTestMetric("testing", 1)
	cache.AddMetric(testMetric)
	assert.Len(t, cache.entries, 1)

	assert.Equal(t, []prometheus.Metric{testMetric}, cache.GetAllMetrics())
	time.Sleep(2 * time.Second)
	assert.Len(t, cache.GetAllMetrics(), 0)
}
