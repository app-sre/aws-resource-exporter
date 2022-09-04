package main

import (
	"context"
	"fmt"
	"math"
	"sync"
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
	maxRetries           = 10
	route53ServiceCode   = "route53"
	hostedZonesQuotaCode = "L-4EA4796A"
)

type Route53Exporter struct {
	sess                       *session.Session
	RecordsPerHostedZoneQuota  *prometheus.Desc
	RecordsPerHostedZoneUsage  *prometheus.Desc
	HostedZonesPerAccountQuota *prometheus.Desc
	HostedZonesPerAccountUsage *prometheus.Desc
	LastUpdateTime             *prometheus.Desc
	Cancel                     context.CancelFunc

	cachedMetrics []prometheus.Metric
	metricsMutex  *sync.Mutex
	logger        log.Logger
	interval      time.Duration
	timeout       time.Duration
}

func NewRoute53Exporter(sess *session.Session, logger log.Logger, interval time.Duration, timeout time.Duration) *Route53Exporter {

	level.Info(logger).Log("msg", "Initializing Route53 exporter")

	exporter := &Route53Exporter{
		sess:                       sess,
		RecordsPerHostedZoneQuota:  prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "route53_recordsperhostedzone_quota"), "Quota for maximum number of records in a Route53 hosted zone", []string{"hostedzoneid", "hostedzonename"}, nil),
		RecordsPerHostedZoneUsage:  prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "route53_recordsperhostedzone_total"), "Number of Resource records", []string{"hostedzoneid", "hostedzonename"}, nil),
		HostedZonesPerAccountQuota: prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "route53_hostedzonesperaccount_quota"), "Quota for maximum number of Route53 hosted zones in an account", []string{}, nil),
		HostedZonesPerAccountUsage: prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "route53_hostedzonesperaccount_total"), "Number of Resource records", []string{}, nil),
		LastUpdateTime:             prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "route53_last_updated_timestamp_seconds"), "Last time, the route53 metrics were sucessfully updated", []string{}, nil),
		cachedMetrics:              []prometheus.Metric{},
		metricsMutex:               &sync.Mutex{},
		logger:                     logger,
		interval:                   interval,
		timeout:                    timeout,
	}
	return exporter
}

func (e *Route53Exporter) getRecordsPerHostedZoneMetrics(client *route53.Route53, hostedZones []*route53.HostedZone, ctx context.Context) ([]prometheus.Metric, []error) {
	metricsChan := make(chan prometheus.Metric, len(hostedZones)*2)
	errChan := make(chan error, len(hostedZones))
	result := []prometheus.Metric{}
	errs := []error{}

	wg := &sync.WaitGroup{}
	wg.Add(len(hostedZones))
	sem := make(chan int, 10)
	defer close(sem)
	for i, hostedZone := range hostedZones {

		sem <- 1
		go func(i int, hostedZone *route53.HostedZone) {
			defer func() {
				<-sem
				wg.Done()
			}()
			hostedZoneLimitOut, err := GetHostedZoneLimitWithBackoff(client, ctx, hostedZone.Id, maxRetries)

			if err != nil {
				errChan <- fmt.Errorf("Could not get Limits for hosted zone with ID '%s' and name '%s'. Error was: %s", *hostedZone.Id, *hostedZone.Name, err.Error())
				level.Info(e.logger).Log("msg", "ERROR")
				exporterMetrics.IncrementErrors()
				return
			}
			level.Info(e.logger).Log("msg", fmt.Sprintf("Currently at hosted zone: %d / %d", i, len(hostedZones)))
			metricsChan <- prometheus.MustNewConstMetric(e.RecordsPerHostedZoneQuota, prometheus.GaugeValue, float64(*hostedZoneLimitOut.Limit.Value), *hostedZone.Id, *hostedZone.Name)
			metricsChan <- prometheus.MustNewConstMetric(e.RecordsPerHostedZoneUsage, prometheus.GaugeValue, float64(*hostedZoneLimitOut.Count), *hostedZone.Id, *hostedZone.Name)

		}(i, hostedZone)
	}
	wg.Wait()
	close(errChan)
	close(metricsChan)

	for metric := range metricsChan {
		result = append(result, metric)
	}
	for err := range errChan {
		errs = append(errs, err)
	}

	return result, errs
}

func (e *Route53Exporter) getHostedZonesPerAccountMetrics(client *servicequotas.ServiceQuotas, hostedZones []*route53.HostedZone, ctx context.Context) ([]prometheus.Metric, error) {
	result := []prometheus.Metric{}
	quota, err := getQuotaValueWithContext(client, route53ServiceCode, hostedZonesQuotaCode, ctx)
	if err != nil {
		return nil, err
	}

	result = append(result,
		prometheus.MustNewConstMetric(e.HostedZonesPerAccountQuota, prometheus.GaugeValue, quota),
		prometheus.MustNewConstMetric(e.HostedZonesPerAccountUsage, prometheus.GaugeValue, float64(len(hostedZones))),
	)
	return result, nil
}

// CollectLoop runs indefinitely to collect the route53 metrics in a cache. Metrics are only written into the cache once all have been collected to ensure that we don't have a partial collect.
func (e *Route53Exporter) CollectLoop() {
	route53Svc := route53.New(e.sess)
	serviceQuotaSvc := servicequotas.New(e.sess)

	for {
		ctx, ctxCancelFunc := context.WithTimeout(context.Background(), e.timeout)
		e.Cancel = ctxCancelFunc
		level.Info(e.logger).Log("msg", "Updating Route53 metrics...")

		hostedZones, err := getAllHostedZones(route53Svc, ctx)

		level.Info(e.logger).Log("msg", "Got all zones")
		if err != nil {
			level.Error(e.logger).Log("msg", "Could not retrieve the list of hosted zones", "error", err.Error())
			exporterMetrics.IncrementErrors()
		}

		allMetrics := []prometheus.Metric{}
		hostedZonesPerAccountMetrics, err := e.getHostedZonesPerAccountMetrics(serviceQuotaSvc, hostedZones, ctx)
		allMetrics = append(allMetrics, hostedZonesPerAccountMetrics...)

		recordsPerHostedZoneMetrics, errs := e.getRecordsPerHostedZoneMetrics(route53Svc, hostedZones, ctx)
		for _, err = range errs {
			level.Error(e.logger).Log("msg", "Could not get limits for hosted zone", "error", err.Error())
			exporterMetrics.IncrementErrors()
		}

		allMetrics = append(allMetrics, recordsPerHostedZoneMetrics...)

		e.metricsMutex.Lock()
		e.cachedMetrics = append(allMetrics, prometheus.MustNewConstMetric(e.LastUpdateTime, prometheus.GaugeValue, float64(time.Now().Unix())))
		e.metricsMutex.Unlock()
		level.Info(e.logger).Log("msg", "Route53 metrics Updated")

		ctxCancelFunc() // should never do anything as we don't run stuff in the background

		time.Sleep(e.interval)
	}
}

func (e *Route53Exporter) Collect(ch chan<- prometheus.Metric) {
	e.metricsMutex.Lock()
	defer e.metricsMutex.Unlock()
	for _, metric := range e.cachedMetrics {
		ch <- metric
	}
}

func (e *Route53Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.RecordsPerHostedZoneQuota
	ch <- e.RecordsPerHostedZoneUsage
	ch <- e.LastUpdateTime
}

func getAllHostedZones(client *route53.Route53, ctx context.Context) ([]*route53.HostedZone, error) {
	result := []*route53.HostedZone{}

	listZonesInput := route53.ListHostedZonesInput{}

	listZonesOut, err := ListHostedZonesWithBackoff(client, ctx, &listZonesInput, maxRetries)
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

func ListHostedZonesWithBackoff(client *route53.Route53, ctx context.Context, input *route53.ListHostedZonesInput, maxTries int) (*route53.ListHostedZonesOutput, error) {
	var listHostedZonesOut *route53.ListHostedZonesOutput
	var err error

	for i := 0; i < maxTries; i++ {
		listHostedZonesOut, err = client.ListHostedZonesWithContext(ctx, input)
		if err == nil {
			return listHostedZonesOut, err
		}
		backOffSeconds := math.Pow(2, float64(i-1))
		fmt.Printf("Backing off for %f.1 seconds\n", backOffSeconds)
		time.Sleep(time.Duration(backOffSeconds) * time.Second)
	}
	return nil, err
}

func GetHostedZoneLimitWithBackoff(client *route53.Route53, ctx context.Context, hostedZoneId *string, maxTries int) (*route53.GetHostedZoneLimitOutput, error) {
	hostedZoneLimitInput := &route53.GetHostedZoneLimitInput{
		HostedZoneId: hostedZoneId,
		Type:         aws.String(route53.HostedZoneLimitTypeMaxRrsetsByZone),
	}
	var hostedZoneLimitOut *route53.GetHostedZoneLimitOutput
	var err error

	for i := 0; i < maxTries; i++ {
		hostedZoneLimitOut, err = client.GetHostedZoneLimitWithContext(ctx, hostedZoneLimitInput)
		if err == nil {
			return hostedZoneLimitOut, err
		}
		backOffSeconds := math.Pow(2, float64(i-1))
		time.Sleep(time.Duration(backOffSeconds) * time.Second)
	}
	return nil, err
}
