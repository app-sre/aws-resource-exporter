package pkg

import (
	"context"
	"log/slog"
	"strings"
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

var ElastiCacheEOLInfos *prometheus.Desc = prometheus.NewDesc(
	prometheus.BuildFQName(namespace, "", "elasticache_eol_info"),
	"The ElastiCache eol date and status for the engine type and version.",
	[]string{"aws_region", "replication_group_id", "engine", "engine_version", "eol_date", "eol_status", "aws_account_id"},
	nil,
)

type ElastiCacheExporter struct {
	configs      []aws.Config
	svcs         []awsclient.Client
	cache        MetricsCache
	awsAccountId string
	eolInfos     []EOLInfo
	thresholds   []Threshold

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
		eolInfos:     config.EOLInfos,
		thresholds:   config.Thresholds,
	}
}

func (e *ElastiCacheExporter) getRegion(configIndex int) string {
	return e.configs[configIndex].Region
}

// AWS publishes ElastiCache EOL dates per major engine version only, while
// DescribeCacheClusters reports the full version (e.g. "6.2.6"), so the lookup is
// keyed on the major version.
func majorEngineVersion(engineVersion string) string {
	major, _, _ := strings.Cut(engineVersion, ".")
	return major
}

// Adds ElastiCache info to metrics cache
func (e *ElastiCacheExporter) addMetricFromElastiCacheInfo(configIndex int, clusters []elasticache_types.CacheCluster, eolInfos []EOLInfo) {
	region := e.getRegion(configIndex)

	var eolMap = make(map[EOLKey]EOLInfo)
	// Engines AWS publishes a schedule for, derived from the EOL data we loaded
	var coveredEngines = make(map[string]bool)
	// Fill eolMap with EOLInfo indexed by engine and major version
	for _, eolinfo := range eolInfos {
		eolMap[EOLKey{Engine: eolinfo.Engine, Version: majorEngineVersion(eolinfo.Version)}] = eolinfo
		coveredEngines[eolinfo.Engine] = true
	}

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

		// Gets EOL for engine and major version
		if eolInfo, ok := eolMap[EOLKey{Engine: engine, Version: majorEngineVersion(engineVersion)}]; ok {
			eolStatus, err := GetEOLStatus(eolInfo.EOL, e.thresholds)
			if err != nil {
				e.logger.Error("Could not get days to ElastiCache EOL for engine version",
					slog.String("engine", engine),
					slog.String("version", engineVersion),
					slog.Any("err", err))
			} else {
				e.cache.AddMetric(prometheus.MustNewConstMetric(ElastiCacheEOLInfos, prometheus.GaugeValue, 1, region, replicationGroupId, engine, engineVersion, eolInfo.EOL, eolStatus, e.awsAccountId))
			}
		} else {
			// A miss means one of two things. If AWS publishes a schedule for this engine
			// (today: Redis OSS only), then a missing version is a gap worth noticing -
			// either AWS added one we have not scraped yet, or the data has gone stale.
			// If the engine has no schedule at all (Valkey, Memcached), the miss is the
			// expected steady state; logging it would just repeat eol_status="unknown"
			// for every cluster on every interval. Either way the status is "unknown"
			// rather than the "red" the RDS exporter uses for a missing date.
			if coveredEngines[engine] {
				e.logger.Info("ElastiCache EOL not found for engine version",
					slog.String("engine", engine),
					slog.String("version", engineVersion))
			}
			e.cache.AddMetric(prometheus.MustNewConstMetric(ElastiCacheEOLInfos, prometheus.GaugeValue, 1, region, replicationGroupId, engine, engineVersion, "no-eol-date", "unknown", e.awsAccountId))
		}
	}
}

func (e *ElastiCacheExporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- RedisVersion
	ch <- ElastiCacheEOLInfos
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
			e.addMetricFromElastiCacheInfo(i, clusters, e.eolInfos)
		}
		e.logger.Info("ElastiCache metrics updated")

		cancel()
		time.Sleep(e.interval)
	}
}
