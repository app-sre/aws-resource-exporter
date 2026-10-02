package pkg

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	elasticache_types "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
)

func createTestCacheClusters() []elasticache_types.CacheCluster {
	return []elasticache_types.CacheCluster{
		{
			CacheClusterId: aws.String("test-cluster"),
			Engine:         aws.String("redis"),
			EngineVersion:  aws.String("123"),
		},
	}
}

func createTestCacheClustersWithEngine(engine string, engineVersion string) []elasticache_types.CacheCluster {
	return []elasticache_types.CacheCluster{
		{
			CacheClusterId:     aws.String("test-cluster"),
			ReplicationGroupId: aws.String("test-replication-group"),
			Engine:             aws.String(engine),
			EngineVersion:      aws.String(engineVersion),
		},
	}
}

func newTestElastiCacheExporter(eolInfos []EOLInfo) ElastiCacheExporter {
	return ElastiCacheExporter{
		configs: []aws.Config{{Region: "foo"}},
		cache:   *NewMetricsCache(10 * time.Second),
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		thresholds: []Threshold{
			{Name: "red", Days: 90},
			{Name: "yellow", Days: 180},
			{Name: "green", Days: 365},
		},
		eolInfos: eolInfos,
	}
}

func TestAddMetricFromElastiCacheInfo(t *testing.T) {
	x := ElastiCacheExporter{
		configs: []aws.Config{{Region: "foo"}},
		cache:   *NewMetricsCache(10 * time.Second),
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	var clusters = []elasticache_types.CacheCluster{}

	x.addMetricFromElastiCacheInfo(0, clusters, nil)
	assert.Len(t, x.cache.GetAllMetrics(), 0)

	// one version metric and one EOL metric per cluster
	x.addMetricFromElastiCacheInfo(0, createTestCacheClusters(), nil)
	assert.Len(t, x.cache.GetAllMetrics(), 2)
}

func TestAddElastiCacheMetricsWithEOLMatch(t *testing.T) {
	// AWS only publishes the major version, the cluster reports the full one
	x := newTestElastiCacheExporter([]EOLInfo{
		{Engine: "redis", Version: "6", EOL: "2000-12-01"},
	})

	x.addMetricFromElastiCacheInfo(0, createTestCacheClustersWithEngine("redis", "6.2.6"), x.eolInfos)

	labels, err := getElastiCacheMetricLabels(&x, ElastiCacheEOLInfos, "eol_date", "eol_status", "engine_version")
	if err != nil {
		t.Errorf("Error retrieving EOL labels: %v", err)
	}

	assert.Equal(t, "2000-12-01", labels["eol_date"])
	assert.Equal(t, "red", labels["eol_status"])
	// the metric keeps the full version, only the lookup is truncated
	assert.Equal(t, "6.2.6", labels["engine_version"])
}

func TestAddElastiCacheMetricsWithoutEOLMatch(t *testing.T) {
	// AWS publishes no EOL dates for valkey
	x := newTestElastiCacheExporter([]EOLInfo{
		{Engine: "redis", Version: "6", EOL: "2000-12-01"},
	})

	x.addMetricFromElastiCacheInfo(0, createTestCacheClustersWithEngine("valkey", "8.1"), x.eolInfos)

	labels, err := getElastiCacheMetricLabels(&x, ElastiCacheEOLInfos, "eol_date", "eol_status")
	if err != nil {
		t.Errorf("Error retrieving EOL labels: %v", err)
	}

	assert.Equal(t, "no-eol-date", labels["eol_date"])
	assert.Equal(t, "unknown", labels["eol_status"])
}

func TestAddElastiCacheMetricsWithMismatchedMajorVersion(t *testing.T) {
	x := newTestElastiCacheExporter([]EOLInfo{
		{Engine: "redis", Version: "6", EOL: "2000-12-01"},
	})

	x.addMetricFromElastiCacheInfo(0, createTestCacheClustersWithEngine("redis", "7.1"), x.eolInfos)

	labels, err := getElastiCacheMetricLabels(&x, ElastiCacheEOLInfos, "eol_date", "eol_status")
	if err != nil {
		t.Errorf("Error retrieving EOL labels: %v", err)
	}

	assert.Equal(t, "no-eol-date", labels["eol_date"])
	assert.Equal(t, "unknown", labels["eol_status"])
}

func TestAddElastiCacheMetricsLogsOnlyUncoveredVersions(t *testing.T) {
	eolInfos := []EOLInfo{{Engine: "redis", Version: "6", EOL: "2000-12-01"}}

	tests := []struct {
		name       string
		engine     string
		version    string
		wantLogged bool
	}{
		{"covered engine and version", "redis", "6.2.6", false},
		{"covered engine, uncovered version", "redis", "7.1", true},
		{"uncovered engine", "valkey", "8.1", false},
		{"uncovered engine", "memcached", "1.6.17", false},
	}

	for _, tt := range tests {
		t.Run(tt.name+" "+tt.engine+" "+tt.version, func(t *testing.T) {
			var logs bytes.Buffer
			x := newTestElastiCacheExporter(eolInfos)
			x.logger = slog.New(slog.NewTextHandler(&logs, nil))

			x.addMetricFromElastiCacheInfo(0, createTestCacheClustersWithEngine(tt.engine, tt.version), eolInfos)

			logged := strings.Contains(logs.String(), "ElastiCache EOL not found")
			assert.Equal(t, tt.wantLogged, logged, "log output: %q", logs.String())

			// the metric is emitted either way
			_, err := getElastiCacheMetricLabels(&x, ElastiCacheEOLInfos, "eol_status")
			assert.NoError(t, err)
		})
	}
}

func TestMajorEngineVersion(t *testing.T) {
	assert.Equal(t, "6", majorEngineVersion("6.2.6"))
	assert.Equal(t, "8", majorEngineVersion("8.1"))
	assert.Equal(t, "6", majorEngineVersion("6"))
	assert.Equal(t, "", majorEngineVersion(""))
}

func getElastiCacheMetricLabels(x *ElastiCacheExporter, metricDesc *prometheus.Desc, labelNames ...string) (map[string]string, error) {
	metricDescription := metricDesc.String()
	metrics := x.cache.GetAllMetrics()

	for _, metric := range metrics {
		if metric.Desc().String() == metricDescription {
			dtoMetric := &dto.Metric{}
			if err := metric.Write(dtoMetric); err != nil {
				return nil, err
			}

			labelValues := make(map[string]string)
			for _, label := range dtoMetric.GetLabel() {
				for _, labelName := range labelNames {
					if label.GetName() == labelName {
						labelValues[labelName] = label.GetValue()
					}
				}
			}

			if len(labelValues) != len(labelNames) {
				return nil, fmt.Errorf("not all requested labels found in metric")
			}

			return labelValues, nil
		}
	}
	return nil, fmt.Errorf("metric not found")
}
