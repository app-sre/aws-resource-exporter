package main

import (
	"context"
	"sync"
	"time"

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

var TransitGatewaysQuota *prometheus.Desc = prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "ec2_transitgateways_quota"), "Quota for maximum number of Transitgateways in this account", []string{"aws_region"}, nil)
var TransitGatewaysUsage *prometheus.Desc = prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "ec2_transitgateways_usage"), "Number of Tranitgatewyas in the AWS Account", []string{"aws_region"}, nil)

type EC2Exporter struct {
	sessions []*session.Session

	logger  log.Logger
	timeout time.Duration
}

func NewEC2Exporter(sessions []*session.Session, logger log.Logger, timeout time.Duration) *EC2Exporter {

	level.Info(logger).Log("msg", "Initializing EC2 exporter")
	return &EC2Exporter{
		sessions: sessions,

		logger:  logger,
		timeout: timeout,
	}
}

func (e *EC2Exporter) Collect(ch chan<- prometheus.Metric) {
	ctx, ctxCancel := context.WithTimeout(context.Background(), e.timeout)
	defer ctxCancel()
	wg := &sync.WaitGroup{}
	wg.Add(len(e.sessions))

	for _, sess := range e.sessions {
		go collectInRegion(sess, e.logger, wg, ch, ctx)
	}
	wg.Wait()
}

func collectInRegion(sess *session.Session, logger log.Logger, wg *sync.WaitGroup, ch chan<- prometheus.Metric, ctx context.Context) {
	defer wg.Done()
	ec2Svc := ec2.New(sess)
	serviceQuotaSvc := servicequotas.New(sess)

	quota, err := getQuotaValueWithContext(serviceQuotaSvc, ec2ServiceCode, transitGatewayPerAccountQuotaCode, ctx)
	if err != nil {
		level.Error(logger).Log("msg", "Could not retrieve Transit Gateway quota", "error", err.Error())
		exporterMetrics.IncrementErrors()
		return
	}

	gateways, err := getAllTransitGatewaysWithContext(ec2Svc, ctx)
	if err != nil {
		level.Error(logger).Log("msg", "Could not retrieve Transit Gateway quota", "error", err.Error())
		exporterMetrics.IncrementErrors()
		return
	}

	ch <- prometheus.MustNewConstMetric(TransitGatewaysUsage, prometheus.GaugeValue, float64(len(gateways)), *sess.Config.Region)
	ch <- prometheus.MustNewConstMetric(TransitGatewaysQuota, prometheus.GaugeValue, quota, *sess.Config.Region)

}

func (e *EC2Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- TransitGatewaysQuota
	ch <- TransitGatewaysUsage
}

func getAllTransitGatewaysWithContext(client *ec2.EC2, ctx context.Context) ([]*ec2.TransitGateway, error) {
	results := []*ec2.TransitGateway{}
	describeGatewaysInput := &ec2.DescribeTransitGatewaysInput{
		DryRun:     aws.Bool(false),
		MaxResults: aws.Int64(1000),
	}

	describeGatewaysOutput, err := client.DescribeTransitGatewaysWithContext(ctx, describeGatewaysInput)

	if err != nil {
		return nil, err
	}
	results = append(results, describeGatewaysOutput.TransitGateways...)

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

func getQuotaValueWithContext(client *servicequotas.ServiceQuotas, serviceCode string, quotaCode string, ctx context.Context) (float64, error) {
	sqOutput, err := client.GetServiceQuotaWithContext(ctx, &servicequotas.GetServiceQuotaInput{
		QuotaCode:   aws.String(quotaCode),
		ServiceCode: aws.String(serviceCode),
	})

	if err != nil {
		return 0, err
	}

	return *sqOutput.Quota.Value, nil
}
