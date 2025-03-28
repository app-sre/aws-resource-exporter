package pkg

import (
	"context"
	"time"

	"github.com/app-sre/aws-resource-exporter/pkg/awsclient"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kafka"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

var MSKInfos *prometheus.Desc = prometheus.NewDesc(
	prometheus.BuildFQName(namespace, "", "msk_eol_info"),
	"The MSK eol date and status for the version.",
	[]string{"aws_region", "cluster_name", "msk_version", "eol_date", "eol_status"},
	nil,
)

type MSKExporter struct {
	sessions     []*session.Session
	svcs         []awsclient.Client
	mskInfos     []MSKInfo
	thresholds   []Threshold
	cache        MetricsCache
	awsAccountId string

	logger   log.Logger
	timeout  time.Duration
	interval time.Duration
}

// NewMSKExporter creates a new MSKExporter instance
func NewMSKExporter(sessions []*session.Session, logger log.Logger, config MSKConfig, awsAccountId string) *MSKExporter {
	level.Info(logger).Log("msg", "Initializing MSK exporter")

	var msks []awsclient.Client
	for _, session := range sessions {
		msks = append(msks, awsclient.NewClientFromSession(session))
	}

	return &MSKExporter{
		sessions:   sessions,
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
	return *e.sessions[sessionIndex].Config.Region
}

func (e *MSKExporter) addMetricFromMSKInfo(sessionIndex int, clusters []*kafka.ClusterInfo, mskInfos []MSKInfo) {
	region := e.getRegion(sessionIndex)

	eolMap := make(map[string]string)
	for _, eolinfo := range mskInfos {
		eolMap[eolinfo.Version] = eolinfo.EOL
	}

	for _, cluster := range clusters {
		clusterName := aws.StringValue(cluster.ClusterName)
		mskVersion := aws.StringValue(cluster.CurrentBrokerSoftwareInfo.KafkaVersion)

		if eolDate, found := eolMap[mskVersion]; found {
			eolStatus, err := GetEOLStatus(eolDate, e.thresholds)
			if err != nil {
				level.Error(e.logger).Log("msg", "Error determining MSK EOL status", "version", mskVersion, "error", err)
			}
			e.cache.AddMetric(prometheus.MustNewConstMetric(MSKInfos, prometheus.GaugeValue, 1, region, clusterName, mskVersion, eolDate, eolStatus))
		} else {
			level.Info(e.logger).Log("msg", "EOL information not found for MSK version %s, setting status to 'unknown'", mskVersion)
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
				level.Error(e.logger).Log("msg", "Call to ListClustersAll failed", "region", *e.sessions[i].Config.Region, "err", err)
				continue
			}
			e.addMetricFromMSKInfo(i, clusters, e.mskInfos)
		}
		level.Info(e.logger).Log("msg", "MSK metrics updated")

		cancel()
		time.Sleep(e.interval)
	}
}
