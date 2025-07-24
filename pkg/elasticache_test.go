package pkg

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	"github.com/stretchr/testify/assert"
)

func createTestCacheClusters() []types.CacheCluster {
	return []types.CacheCluster{
		{
			CacheClusterId: aws.String("test-cluster"),
			Engine:         aws.String("redis"),
			EngineVersion:  aws.String("123"),
		},
	}
}

func TestAddMetricFromElastiCacheInfo(t *testing.T) {
	x := ElastiCacheExporter{
		cfgs:   []aws.Config{{Region: "foo"}},
		cache:  *NewMetricsCache(10 * time.Second),
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	var clusters = []types.CacheCluster{}

	x.addMetricFromElastiCacheInfo(0, clusters)
	assert.Len(t, x.cache.GetAllMetrics(), 0)

	x.addMetricFromElastiCacheInfo(0, createTestCacheClusters())
	assert.Len(t, x.cache.GetAllMetrics(), 1)
}
