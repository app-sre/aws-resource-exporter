package main

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/aws/aws-sdk-go/service/servicequotas"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	route53ServiceCode   = "route53"
	hostedZonesQuotaCode = "L-4EA4796A"
)

type Route53Exporter struct {
	sess                       *session.Session
	HostedZonesPerAccountQuota *prometheus.Desc
	HostedZonesPerAccountUsage *prometheus.Desc

	logger  log.Logger
	timeout time.Duration
}

func NewRoute53Exporter(sess *session.Session, logger log.Logger, timeout time.Duration) *Route53Exporter {

	level.Info(logger).Log("msg", "Initializing Route53 exporter")

	return &Route53Exporter{
		sess:                       sess,
		HostedZonesPerAccountQuota: prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "route53_hostedzonesperaccount_quota"), "Quota for maximum number of Route53 hosted zones in an account", []string{}, nil),
		HostedZonesPerAccountUsage: prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "route53_hostedzonesperaccount_total"), "Number of Resource records", []string{}, nil),
		logger:                     logger,
		timeout:                    timeout,
	}
}

func (e *Route53Exporter) Collect(ch chan<- prometheus.Metric) {
	ctx, ctxCancel := context.WithTimeout(context.Background(), e.timeout)
	defer ctxCancel()
	route53Svc := route53.New(e.sess)
	serviceQuotaSvc := servicequotas.New(e.sess)

	hostedZones, err := getAllHostedZones(route53Svc, ctx)
	if err != nil {
		level.Error(e.logger).Log("msg", "Could not retrieve the list of hosted zones", "error", err.Error())
		exporterMetrics.IncrementErrors()
		return
	}

	hostedZonesQuota, err := getQuotaValueWithContext(serviceQuotaSvc, route53ServiceCode, hostedZonesQuotaCode, ctx)
	if err != nil {
		level.Error(e.logger).Log("msg", "Could not retrieve Hosted zones quota", "error", err.Error())
		exporterMetrics.IncrementErrors()
		return
	}

	ch <- prometheus.MustNewConstMetric(e.HostedZonesPerAccountQuota, prometheus.GaugeValue, float64(len(hostedZones)))
	ch <- prometheus.MustNewConstMetric(e.HostedZonesPerAccountUsage, prometheus.GaugeValue, hostedZonesQuota)
}

func getAllHostedZones(client *route53.Route53, ctx context.Context) ([]*route53.HostedZone, error) {
	result := []*route53.HostedZone{}

	listZonesInput := route53.ListHostedZonesInput{}

	listZonesOut, err := client.ListHostedZonesWithContext(ctx, &listZonesInput)
	if err != nil {
		return nil, err
	}
	result = append(result, listZonesOut.HostedZones...)

	for *listZonesOut.IsTruncated {
		listZonesInput.Marker = listZonesOut.NextMarker
		listZonesOut, err = client.ListHostedZonesWithContext(ctx, &listZonesInput)
		if err != nil {
			return nil, err
		}
		result = append(result, listZonesOut.HostedZones...)
	}

	return result, nil
}

func (e *Route53Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.HostedZonesPerAccountQuota
	ch <- e.HostedZonesPerAccountUsage
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
