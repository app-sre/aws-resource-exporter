package pkg

import (
	"context"
	"log/slog"
	"time"

	"github.com/app-sre/aws-resource-exporter/pkg/awsclient"
	"github.com/aws/aws-sdk-go-v2/aws"
	elasticache_types "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	"github.com/prometheus/client_golang/prometheus"
)

var RedisVersion *prometheus.Desc = prometheus.NewDesc(
	prometheus.BuildFQName(namespace, "", "elasticache_redisversion"),
	"The ElastiCache engine type and version.",
	[]string{"aws_region", "replication_group_id", "engine", "engine_version", "aws_account_id"},
	nil,
)

type ElastiCacheExporter struct {
	configs      []aws.Config
	svcs         []awsclient.Client
	cache        MetricsCache
	awsAccountId string

	logger   *slog.Logger
	timeout  time.Duration
	interval time.Duration
}

// NewElastiCacheExporter creates a new ElastiCacheExporter instance
func NewElastiCacheExporter(configs []aws.Config, logger *slog.Logger, config ElastiCacheConfig, awsAccountId string) *ElastiCacheExporter {
	logger.Info("Initializing ElastiCache exporter")

	var elasticaches []awsclient.Client
	for _, cfg := range configs {
		elasticaches = append(elasticaches, awsclient.NewClientFromConfig(cfg))
	}

	return &ElastiCacheExporter{
		configs:      configs,
		svcs:         elasticaches,
		cache:        *NewMetricsCache(*config.CacheTTL),
		logger:       logger,
		timeout:      *config.Timeout,
		interval:     *config.Interval,
		awsAccountId: awsAccountId,
	}
}

func (e *ElastiCacheExporter) getRegion(configIndex int) string {
	return e.configs[configIndex].Region
}

// Adds ElastiCache info to metrics cache
func (e *ElastiCacheExporter) addMetricFromElastiCacheInfo(configIndex int, clusters []elasticache_types.CacheCluster) {
	region := e.getRegion(configIndex)

	for _, cluster := range clusters {
		replicationGroupId := ""
		if cluster.ReplicationGroupId != nil {
			replicationGroupId = *cluster.ReplicationGroupId
		}
		engine := ""
		if cluster.Engine != nil {
			engine = *cluster.Engine
		}
		engineVersion := ""
		if cluster.EngineVersion != nil {
			engineVersion = *cluster.EngineVersion
		}

		e.cache.AddMetric(prometheus.MustNewConstMetric(RedisVersion, prometheus.GaugeValue, 1, region, replicationGroupId, engine, engineVersion, e.awsAccountId))
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
				e.logger.Error("Call to DescribeCacheClustersAll failed",
					slog.String("region", e.configs[i].Region),
					slog.Any("err", err))
				awsclient.AwsExporterMetrics.IncrementErrors()
				continue
			}
			e.addMetricFromElastiCacheInfo(i, clusters)
		}
		e.logger.Info("ElastiCache metrics updated")

		cancel()
		time.Sleep(e.interval)
	}
}
