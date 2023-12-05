package pkg

import (
	"context"
	"time"

	"github.com/app-sre/aws-resource-exporter/pkg/awsclient"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

var RedisVersion *prometheus.Desc = prometheus.NewDesc(
	prometheus.BuildFQName(namespace, "", "elasticache_redisversion"),
	"The ElastiCache engine type and version.",
	[]string{"aws_region", "cache_name", "engine", "engine_version", "aws_account_id"},
	nil,
)

type ElastiCacheExporter struct {
	sessions     []*session.Session
	svcs         []awsclient.Client
	cache        MetricsCache
	awsAccountId string

	logger   log.Logger
	timeout  time.Duration
	interval time.Duration
}

// NewElastiCacheExporter creates a new ElastiCacheExporter instance
func NewElastiCacheExporter(sessions []*session.Session, logger log.Logger, config ElastiCacheConfig, awsAccountId string) *ElastiCacheExporter {
	level.Info(logger).Log("msg", "Initializing ElastiCache exporter")

	var elasticaches []awsclient.Client
	for _, session := range sessions {
		elasticaches = append(elasticaches, awsclient.NewClientFromSession(session))
	}

	return &ElastiCacheExporter{
		sessions:     sessions,
		svcs:         elasticaches,
		cache:        *NewMetricsCache(*config.CacheTTL),
		logger:       logger,
		timeout:      *config.Timeout,
		interval:     *config.Interval,
		awsAccountId: awsAccountId,
	}
}

func (e *ElastiCacheExporter) getRegion(sessionIndex int) string {
	return *e.sessions[sessionIndex].Config.Region
}

// Adds ElastiCache info to metrics cache
func (e *ElastiCacheExporter) addMetricFromElastiCacheInfo(sessionIndex int, clusters []*elasticache.CacheCluster) {
	region := e.getRegion(sessionIndex)

	for _, cluster := range clusters {
		cacheName := aws.StringValue(cluster.CacheClusterId)
		engine := aws.StringValue(cluster.Engine)
		engineVersion := aws.StringValue(cluster.EngineVersion)

		e.cache.AddMetric(prometheus.MustNewConstMetric(RedisVersion, prometheus.GaugeValue, 1, region, cacheName, engine, engineVersion, e.awsAccountId))
	}
}

func (e *ElastiCacheExporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- RedisVersion
}

func (e *ElastiCacheExporter) Collect(ch chan<- prometheus.Metric) {
	for _, m := range e.cache.GetAllMetrics() {
		ch <- m
	}
}

func (e *ElastiCacheExporter) CollectLoop() {
	for {
		ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
		for i, client := range e.svcs {
			clusters, err := client.DescribeCacheClustersAll(ctx)
			if err != nil {
				level.Error(e.logger).Log("msg", "Call to DescribeCacheClustersAll failed", "region", *e.sessions[i].Config.Region, "err", err)
				continue
			}
			e.addMetricFromElastiCacheInfo(i, clusters)
		}
		level.Info(e.logger).Log("msg", "ElastiCache metrics updated")

		cancel()
		time.Sleep(e.interval)
	}
}
