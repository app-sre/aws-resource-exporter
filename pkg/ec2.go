package pkg

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/app-sre/aws-resource-exporter/pkg/awsclient"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2_types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/servicequotas"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	transitGatewayPerAccountQuotaCode string = "L-A2478D36"
	ec2ServiceCode                    string = "ec2"
)

var TransitGatewaysQuota *prometheus.Desc
var TransitGatewaysUsage *prometheus.Desc
var EC2BandwidthLimitGbps *prometheus.Desc

type EC2Exporter struct {
	configs          []aws.Config
	cache            MetricsCache
	bandwidthEnabled bool

	logger   *slog.Logger
	timeout  time.Duration
	interval time.Duration
}

func NewEC2Exporter(configs []aws.Config, logger *slog.Logger, config EC2Config, awsAccountId string) *EC2Exporter {

	logger.Info("Initializing EC2 exporter")
	constLabels := map[string]string{"aws_account_id": awsAccountId, QUOTA_CODE_KEY: transitGatewayPerAccountQuotaCode, SERVICE_CODE_KEY: ec2ServiceCode}

	TransitGatewaysQuota = prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "ec2_transitgatewaysperregion_quota"), "Quota for maximum number of Transitgateways in this account", []string{"aws_region"}, constLabels)
	TransitGatewaysUsage = prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "ec2_transitgatewaysperregion_usage"), "Number of Tranitgatewyas in the AWS Account", []string{"aws_region"}, constLabels)
	EC2BandwidthLimitGbps = prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "ec2_instance_bandwidth_limit_gbps"), "Network bandwidth limit in Gbps for an EC2 instance", []string{"aws_region", "instance_id", "instance_type"}, map[string]string{"aws_account_id": awsAccountId})

	return &EC2Exporter{
		configs:          configs,
		cache:            *NewMetricsCache(*config.CacheTTL),
		bandwidthEnabled: config.BandwidthMetrics,

		logger:   logger,
		timeout:  *config.Timeout,
		interval: *config.Interval,
	}
}

func (e *EC2Exporter) Collect(ch chan<- prometheus.Metric) {
	for _, m := range e.cache.GetAllMetrics() {
		ch <- m
	}
}

func (e *EC2Exporter) CollectLoop() {
	for {
		func() {
			ctx, ctxCancel := context.WithTimeout(context.Background(), e.timeout)
			defer ctxCancel()
			wg := &sync.WaitGroup{}
			wg.Add(len(e.configs))

			for _, cfg := range e.configs {
				go e.collectInRegion(cfg, e.logger, wg, ctx)
			}
			wg.Wait()
		}()

		e.logger.Info("EC2 metrics Updated")

		time.Sleep(e.interval)
	}
}

func (e *EC2Exporter) collectInRegion(cfg aws.Config, logger *slog.Logger, wg *sync.WaitGroup, ctx context.Context) {
	defer wg.Done()

	aws := awsclient.NewClientFromConfig(cfg)

	quota, err := getQuotaValueWithContext(aws, ec2ServiceCode, transitGatewayPerAccountQuotaCode, ctx)
	if err != nil {
		logger.Error("Could not retrieve Transit Gateway quota", slog.String("error", err.Error()))
		awsclient.AwsExporterMetrics.IncrementErrors()
		return
	}

	count, err := getTransitGatewaysCountWithContext(aws, ctx)
	if err != nil {
		logger.Error("Could not retrieve Transit Gateway count", slog.String("error", err.Error()))
		awsclient.AwsExporterMetrics.IncrementErrors()
		return
	}

	e.cache.AddMetric(prometheus.MustNewConstMetric(TransitGatewaysUsage, prometheus.GaugeValue, float64(count), cfg.Region))
	e.cache.AddMetric(prometheus.MustNewConstMetric(TransitGatewaysQuota, prometheus.GaugeValue, quota, cfg.Region))

	// Collect instance bandwidth metrics if enabled
	if e.bandwidthEnabled {
		e.collectInstanceBandwidth(aws, cfg.Region, logger, ctx)
	}
}

func (e *EC2Exporter) collectInstanceBandwidth(client awsclient.Client, region string, logger *slog.Logger, ctx context.Context) {
	instances, err := client.DescribeInstancesAll(ctx)
	if err != nil {
		logger.Error("Could not retrieve EC2 instances", slog.String("region", region), slog.String("error", err.Error()))
		awsclient.AwsExporterMetrics.IncrementErrors()
		return
	}

	if len(instances) == 0 {
		return
	}

	// Collect unique instance types
	instanceTypeSet := make(map[ec2_types.InstanceType]struct{})
	for _, inst := range instances {
		instanceTypeSet[inst.InstanceType] = struct{}{}
	}
	instanceTypes := make([]ec2_types.InstanceType, 0, len(instanceTypeSet))
	for it := range instanceTypeSet {
		instanceTypes = append(instanceTypes, it)
	}

	// Fetch instance type info (bandwidth data)
	typeInfos, err := client.DescribeInstanceTypes(ctx, instanceTypes)
	if err != nil {
		logger.Error("Could not retrieve EC2 instance type info", slog.String("region", region), slog.String("error", err.Error()))
		awsclient.AwsExporterMetrics.IncrementErrors()
		return
	}

	// Build lookup: instance type -> baseline bandwidth in Gbps
	bandwidthMap := make(map[ec2_types.InstanceType]float64)
	for _, info := range typeInfos {
		if info.NetworkInfo != nil && info.NetworkInfo.NetworkCards != nil && len(info.NetworkInfo.NetworkCards) > 0 {
			// Use BaselineBandwidthInGbps if available, otherwise PeakBandwidthInGbps
			if info.NetworkInfo.NetworkCards[0].BaselineBandwidthInGbps != nil {
				bandwidthMap[info.InstanceType] = *info.NetworkInfo.NetworkCards[0].BaselineBandwidthInGbps
			} else if info.NetworkInfo.NetworkCards[0].PeakBandwidthInGbps != nil {
				bandwidthMap[info.InstanceType] = *info.NetworkInfo.NetworkCards[0].PeakBandwidthInGbps
			}
		}
	}

	// Emit a metric per instance
	for _, inst := range instances {
		instanceId := ""
		if inst.InstanceId != nil {
			instanceId = *inst.InstanceId
		}
		instanceType := string(inst.InstanceType)

		bw, ok := bandwidthMap[inst.InstanceType]
		if !ok {
			continue
		}

		e.cache.AddMetric(prometheus.MustNewConstMetric(
			EC2BandwidthLimitGbps,
			prometheus.GaugeValue,
			bw,
			region, instanceId, instanceType,
		))
	}
}

func (e *EC2Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- TransitGatewaysQuota
	ch <- TransitGatewaysUsage
	ch <- EC2BandwidthLimitGbps
}

func getTransitGatewaysCountWithContext(client awsclient.Client, ctx context.Context) (int, error) {
	input := &ec2.DescribeTransitGatewaysInput{
		DryRun:     aws.Bool(false),
		MaxResults: aws.Int32(1000),
	}
	return client.GetTransitGatewaysCount(ctx, input)
}

func createGetServiceQuotaInput(serviceCode, quotaCode string) *servicequotas.GetServiceQuotaInput {
	return &servicequotas.GetServiceQuotaInput{
		ServiceCode: aws.String(serviceCode),
		QuotaCode:   aws.String(quotaCode),
	}
}

func getQuotaValueWithContext(client awsclient.Client, serviceCode string, quotaCode string, ctx context.Context) (float64, error) {
	sqOutput, err := client.GetServiceQuota(ctx, createGetServiceQuotaInput(serviceCode, quotaCode))

	if err != nil {
		return 0, err
	}

	if sqOutput.Quota == nil || sqOutput.Quota.Value == nil {
		return 0, fmt.Errorf("quota value not found for servicecode %s and quotacode %s", serviceCode, quotaCode)
	}

	return *sqOutput.Quota.Value, nil
}
