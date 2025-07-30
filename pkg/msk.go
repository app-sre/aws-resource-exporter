package pkg

import (
	"context"
	"log/slog"
	"time"

	"github.com/app-sre/aws-resource-exporter/pkg/awsclient"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kafka/types"
	"github.com/prometheus/client_golang/prometheus"
)

var MSKInfos *prometheus.Desc = prometheus.NewDesc(
	prometheus.BuildFQName(namespace, "", "msk_eol_info"),
	"The MSK eol date and status for the version.",
	[]string{"aws_region", "cluster_name", "msk_version", "eol_date", "eol_status"},
	nil,
)

type MSKExporter struct {
	cfgs         []aws.Config
	svcs         []awsclient.Client
	mskInfos     []MSKInfo
	thresholds   []Threshold
	cache        MetricsCache
	awsAccountId string

	logger   *slog.Logger
	timeout  time.Duration
	interval time.Duration
}

// NewMSKExporter creates a new MSKExporter instance
func NewMSKExporter(cfgs []aws.Config, logger *slog.Logger, config MSKConfig, awsAccountId string) *MSKExporter {
	logger.Info("Initializing MSK exporter")

	var msks []awsclient.Client
	for _, cfg := range cfgs {
		msks = append(msks, awsclient.NewClientFromConfig(cfg))
	}

	return &MSKExporter{
		cfgs:       cfgs,
		svcs:       msks,
		cache:      *NewMetricsCache(*config.CacheTTL),
		logger:     logger,
		timeout:    *config.Timeout,
		interval:   *config.Interval,
		mskInfos:   config.MSKInfos,
		thresholds: config.Thresholds,
	}
}

func (e *MSKExporter) getRegion(sessionIndex int) string {
	return e.cfgs[sessionIndex].Region
}

func (e *MSKExporter) addMetricFromMSKInfo(sessionIndex int, clusters []types.ClusterInfo, mskInfos []MSKInfo) {
	region := e.getRegion(sessionIndex)

	eolMap := make(map[string]string)
	for _, eolinfo := range mskInfos {
		eolMap[eolinfo.Version] = eolinfo.EOL
	}

	for _, cluster := range clusters {
		clusterName := aws.ToString(cluster.ClusterName)
		mskVersion := aws.ToString(cluster.CurrentBrokerSoftwareInfo.KafkaVersion)

		if eolDate, found := eolMap[mskVersion]; found {
			eolStatus, err := GetEOLStatus(eolDate, e.thresholds)
			if err != nil {
				e.logger.Error("Error determining MSK EOL status", slog.String("version", mskVersion), slog.Any("error", err))

			}
			e.cache.AddMetric(prometheus.MustNewConstMetric(MSKInfos, prometheus.GaugeValue, 1, region, clusterName, mskVersion, eolDate, eolStatus))
		} else {
			e.logger.Info("msg", "EOL information not found for MSK version %s, setting status to 'unknown'", mskVersion)
			e.cache.AddMetric(prometheus.MustNewConstMetric(MSKInfos, prometheus.GaugeValue, 1, region, clusterName, mskVersion, "no-eol-date", "unknown"))
		}
	}
}

func (e *MSKExporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- MSKInfos
}

func (e *MSKExporter) Collect(ch chan<- prometheus.Metric) {
	for _, m := range e.cache.GetAllMetrics() {
		ch <- m
	}
}

func (e *MSKExporter) CollectLoop() {
	for {
		ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
		for i, svc := range e.svcs {
			clusters, err := svc.ListClustersAll(ctx)
			if err != nil {
				e.logger.Error("Call to ListClustersAll failed", slog.String("region", e.cfgs[i].Region), slog.Any("err", err))
				continue
			}
			e.addMetricFromMSKInfo(i, clusters, e.mskInfos)
		}
		e.logger.Info("MSK metrics updated")

		cancel()
		time.Sleep(e.interval)
	}
}
