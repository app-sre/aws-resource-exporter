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
			EngineVersion:  aws.String("123"),
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
