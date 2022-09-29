package pkg

import (
	"context"
	"sync"
	"time"

	"github.com/app-sre/aws-resource-exporter/pkg/awsclient"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/servicequotas"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	transitGatewayPerAccountQuotaCode string = "L-A2478D36"
	ec2ServiceCode                    string = "ec2"
)

var TransitGatewaysQuota *prometheus.Desc
var TransitGatewaysUsage *prometheus.Desc

type EC2Exporter struct {
	sessions []*session.Session
	cache    MetricsCache

	logger   log.Logger
	timeout  time.Duration
	interval time.Duration
}

func NewEC2Exporter(sessions []*session.Session, logger log.Logger, config EC2Config, awsAccountId string) *EC2Exporter {

	level.Info(logger).Log("msg", "Initializing EC2 exporter")
	constLabels := map[string]string{"aws_account_id": awsAccountId, QUOTA_CODE_KEY: transitGatewayPerAccountQuotaCode, SERVICE_CODE_KEY: ec2ServiceCode}

	TransitGatewaysQuota = prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "ec2_transitgatewaysperregion_quota"), "Quota for maximum number of Transitgateways in this account", []string{"aws_region"}, constLabels)
	TransitGatewaysUsage = prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "ec2_transitgatewaysperregion_usage"), "Number of Tranitgatewyas in the AWS Account", []string{"aws_region"}, constLabels)

	return &EC2Exporter{
		sessions: sessions,
		cache:    *NewMetricsCache(*config.CacheTTL),

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
		ctx, ctxCancel := context.WithTimeout(context.Background(), e.timeout)
		defer ctxCancel()
		wg := &sync.WaitGroup{}
		wg.Add(len(e.sessions))

		for _, sess := range e.sessions {
			go e.collectInRegion(sess, e.logger, wg, ctx)
		}
		wg.Wait()

		level.Info(e.logger).Log("msg", "EC2 metrics Updated")

		time.Sleep(e.interval)
	}
}

func (e *EC2Exporter) collectInRegion(sess *session.Session, logger log.Logger, wg *sync.WaitGroup, ctx context.Context) {
	defer wg.Done()

	aws := awsclient.NewClientFromSession(sess)

	quota, err := getQuotaValueWithContext(aws, ec2ServiceCode, transitGatewayPerAccountQuotaCode, ctx)
	if err != nil {
		level.Error(logger).Log("msg", "Could not retrieve Transit Gateway quota", "error", err.Error())
		awsclient.AwsExporterMetrics.IncrementErrors()
		return
	}

	gateways, err := getAllTransitGatewaysWithContext(aws, ctx)
	if err != nil {
		level.Error(logger).Log("msg", "Could not retrieve Transit Gateway quota", "error", err.Error())
		awsclient.AwsExporterMetrics.IncrementErrors()
		return
	}

	e.cache.AddMetric(prometheus.MustNewConstMetric(TransitGatewaysUsage, prometheus.GaugeValue, float64(len(gateways)), *sess.Config.Region))
	e.cache.AddMetric(prometheus.MustNewConstMetric(TransitGatewaysQuota, prometheus.GaugeValue, quota, *sess.Config.Region))
}

func (e *EC2Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- TransitGatewaysQuota
	ch <- TransitGatewaysUsage
}

func createDescribeTransitGatewayInput() *ec2.DescribeTransitGatewaysInput {
	return &ec2.DescribeTransitGatewaysInput{
		DryRun:     aws.Bool(false),
		MaxResults: aws.Int64(1000),
	}
}

func createGetServiceQuotaInput(serviceCode, quotaCode string) *servicequotas.GetServiceQuotaInput {
	return &servicequotas.GetServiceQuotaInput{
		ServiceCode: aws.String(serviceCode),
		QuotaCode:   aws.String(quotaCode),
	}
}

func getAllTransitGatewaysWithContext(client awsclient.Client, ctx context.Context) ([]*ec2.TransitGateway, error) {
	results := []*ec2.TransitGateway{}
	describeGatewaysInput := createDescribeTransitGatewayInput()
	describeGatewaysOutput, err := client.DescribeTransitGatewaysWithContext(ctx, describeGatewaysInput)

	if err != nil {
		return nil, err
	}
	results = append(results, describeGatewaysOutput.TransitGateways...)
	// TODO: replace with aws-go-sdk pagination method
	for describeGatewaysOutput.NextToken != nil {
		describeGatewaysInput.SetNextToken(*describeGatewaysOutput.NextToken)
		describeGatewaysOutput, err := client.DescribeTransitGatewaysWithContext(ctx, describeGatewaysInput)
		if err != nil {
			return nil, err
		}
		results = append(results, describeGatewaysOutput.TransitGateways...)
	}

	return results, nil
}

func getQuotaValueWithContext(client awsclient.Client, serviceCode string, quotaCode string, ctx context.Context) (float64, error) {
	sqOutput, err := client.GetServiceQuotaWithContext(ctx, createGetServiceQuotaInput(serviceCode, quotaCode))

	if err != nil {
		return 0, err
	}

	return *sqOutput.Quota.Value, nil
}
