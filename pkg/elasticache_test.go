package pkg

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	elasticache_types "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	"github.com/stretchr/testify/assert"
)

func createTestCacheClusters() []elasticache_types.CacheCluster {
	return []elasticache_types.CacheCluster{
		{
			CacheClusterId: aws.String("test-cluster"),
			Engine:         aws.String("redis"),
			EngineVersion:  aws.String("6.2"),
		},
	}
}

func createTestCacheClustersValkey() []elasticache_types.CacheCluster {
	return []elasticache_types.CacheCluster{
		{
			CacheClusterId:     aws.String("valkey-cluster"),
			ReplicationGroupId: aws.String("valkey-rg"),
			Engine:             aws.String("redis"),
			EngineVersion:      aws.String("7.2"),
		},
	}
}

func TestAddMetricFromElastiCacheInfo(t *testing.T) {
	x := ElastiCacheExporter{
		configs: []aws.Config{{Region: "foo"}},
		cache:   *NewMetricsCache(10 * time.Second),
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	var clusters = []elasticache_types.CacheCluster{}

	x.addMetricFromElastiCacheInfo(0, clusters)
	assert.Len(t, x.cache.GetAllMetrics(), 0)

	x.addMetricFromElastiCacheInfo(0, createTestCacheClusters())
	assert.Len(t, x.cache.GetAllMetrics(), 1)
}

func TestAddMetricFromElastiCacheInfoValkey(t *testing.T) {
	x := ElastiCacheExporter{
		configs: []aws.Config{{Region: "us-east-1"}},
		cache:   *NewMetricsCache(10 * time.Second),
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	x.addMetricFromElastiCacheInfo(0, createTestCacheClustersValkey())
	assert.Len(t, x.cache.GetAllMetrics(), 1)
}

func TestResolveElastiCacheEngine(t *testing.T) {
	tests := []struct {
		name           string
		reportedEngine string
		engineVersion  string
		expected       string
	}{
		{"redis 6.2 stays redis", "redis", "6.2", "redis"},
		{"redis 7.1 stays redis", "redis", "7.1", "redis"},
		{"redis 7.2 becomes valkey", "redis", "7.2", "valkey"},
		{"redis 7.3 becomes valkey", "redis", "7.3", "valkey"},
		{"redis 8.0 becomes valkey", "redis", "8.0", "valkey"},
		{"valkey 7.2 stays valkey", "valkey", "7.2", "valkey"},
		{"valkey 8.0 stays valkey", "valkey", "8.0", "valkey"},
		{"invalid version keeps reported", "redis", "invalid", "redis"},
		{"empty version keeps reported", "redis", "", "redis"},
		{"major only redis 6", "redis", "6", "redis"},
		{"patch version 7.2.1 becomes valkey", "redis", "7.2.1", "valkey"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveElastiCacheEngine(tt.reportedEngine, tt.engineVersion)
			assert.Equal(t, tt.expected, result)
		})
	}
}
