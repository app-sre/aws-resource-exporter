package pkg

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/go-kit/log"
	"github.com/stretchr/testify/assert"
)

func createTestCacheClusters() []*elasticache.CacheCluster {
	return []*elasticache.CacheCluster{
		{
			CacheClusterId: aws.String("test-cluster"),
			Engine:         aws.String("redis"),
			EngineVersion:  aws.String("123"),
		},
	}
}

func TestAddMetricFromElastiCacheInfo(t *testing.T) {
	x := ElastiCacheExporter{
		sessions: []*session.Session{session.New(&aws.Config{Region: aws.String("foo")})},
		cache:    *NewMetricsCache(10 * time.Second),
		logger:   log.NewNopLogger(),
	}

	var clusters = []*elasticache.CacheCluster{}

	x.addMetricFromElastiCacheInfo(0, clusters)
	assert.Len(t, x.cache.GetAllMetrics(), 0)

	x.addMetricFromElastiCacheInfo(0, createTestCacheClusters())
	assert.Len(t, x.cache.GetAllMetrics(), 1)
}
