package main

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

type Route53Exporter struct {
	sess                      *session.Session
	RecordsPerHostedZoneQuota *prometheus.Desc
	RecordsPerHostedZoneUsage *prometheus.Desc

	logger  log.Logger
	timeout time.Duration
}

func NewRoute53Exporter(sess *session.Session, logger log.Logger, timeout time.Duration) *Route53Exporter {

	level.Info(logger).Log("msg", "Initializing Route53 exporter")

	return &Route53Exporter{
		sess:                      sess,
		RecordsPerHostedZoneQuota: prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "route53_recordsperhostedzone_quota"), "Quota for maximum number of records in a Route53 hosted zone", []string{"hostedzoneid"}, nil),
		RecordsPerHostedZoneUsage: prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "route53_recordsperhostedzone_total"), "Number of Resource records", []string{"hostedzoneid"}, nil),
		logger:                    logger,
		timeout:                   timeout,
	}
}

func (e *Route53Exporter) Collect(ch chan<- prometheus.Metric) {
	ctx, ctxCancel := context.WithTimeout(context.Background(), e.timeout)
	defer ctxCancel()
	route53Svc := route53.New(e.sess)

	hostedZones, err := getAllHostedZones(route53Svc, ctx)
	if err != nil {
		level.Error(e.logger).Log("msg", "Could not retrieve the list of hosted zones", "error", err.Error())
		exporterMetrics.IncrementErrors()
	}

	for _, hostedZone := range hostedZones {
		hostedZoneLimitOut, err := route53Svc.GetHostedZoneLimitWithContext(ctx, &route53.GetHostedZoneLimitInput{
			HostedZoneId: hostedZone.Id,
			Type:         aws.String(route53.HostedZoneLimitTypeMaxRrsetsByZone),
		})

		if err != nil {
			level.Error(e.logger).Log("msg", "Could not get Quota for hosted zone", "hostedZoneId", hostedZone.Id, "error", err.Error())
			exporterMetrics.IncrementErrors()
			continue
		}

		ch <- prometheus.MustNewConstMetric(e.RecordsPerHostedZoneQuota, prometheus.GaugeValue, float64(*hostedZoneLimitOut.Limit.Value), *hostedZone.Id)
		ch <- prometheus.MustNewConstMetric(e.RecordsPerHostedZoneUsage, prometheus.GaugeValue, float64(*hostedZoneLimitOut.Count), *hostedZone.Id)
	}
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
		listZonesInput.Marker = listZonesOut.Marker
		listZonesOut, err = client.ListHostedZonesWithContext(ctx, &listZonesInput)
		if err != nil {
			return nil, err
		}
		result = append(result, listZonesOut.HostedZones...)
	}

	return result, nil
}

func (e *Route53Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.RecordsPerHostedZoneQuota
	ch <- e.RecordsPerHostedZoneUsage
}
