package pkg

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/app-sre/aws-resource-exporter/pkg/awsclient"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	maxRetries                    = 10
	route53MaxConcurrency         = 5
	route53ServiceCode            = "route53"
	hostedZonesQuotaCode          = "L-4EA4796A"
	recordsPerHostedZoneQuotaCode = "L-E209CC9F"
	errorCodeThrottling           = "Throttling"
)

type Route53Exporter struct {
	sess                       *session.Session
	RecordsPerHostedZoneQuota  *prometheus.Desc
	RecordsPerHostedZoneUsage  *prometheus.Desc
	HostedZonesPerAccountQuota *prometheus.Desc
	HostedZonesPerAccountUsage *prometheus.Desc
	LastUpdateTime             *prometheus.Desc
	Cancel                     context.CancelFunc

	cache    MetricsCache
	logger   *slog.Logger
	interval time.Duration
	timeout  time.Duration
}

func NewRoute53Exporter(sess *session.Session, logger *slog.Logger, config Route53Config, awsAccountId string) *Route53Exporter {

	logger.Info("msg", "Initializing Route53 exporter")
	constLabels := map[string]string{"aws_account_id": awsAccountId, SERVICE_CODE_KEY: route53ServiceCode}

	exporter := &Route53Exporter{
		sess:                       sess,
		RecordsPerHostedZoneQuota:  prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "route53_recordsperhostedzone_quota"), "Quota for maximum number of records in a Route53 hosted zone", []string{"hostedzoneid", "hostedzonename"}, WithKeyValue(constLabels, QUOTA_CODE_KEY, recordsPerHostedZoneQuotaCode)),
		RecordsPerHostedZoneUsage:  prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "route53_recordsperhostedzone_total"), "Number of Resource records", []string{"hostedzoneid", "hostedzonename"}, WithKeyValue(constLabels, QUOTA_CODE_KEY, recordsPerHostedZoneQuotaCode)),
		HostedZonesPerAccountQuota: prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "route53_hostedzonesperaccount_quota"), "Quota for maximum number of Route53 hosted zones in an account", []string{}, WithKeyValue(constLabels, QUOTA_CODE_KEY, hostedZonesQuotaCode)),
		HostedZonesPerAccountUsage: prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "route53_hostedzonesperaccount_total"), "Number of Resource records", []string{}, WithKeyValue(constLabels, QUOTA_CODE_KEY, hostedZonesQuotaCode)),
		LastUpdateTime:             prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "route53_last_updated_timestamp_seconds"), "Last time, the route53 metrics were sucessfully updated", []string{}, constLabels),
		cache:                      *NewMetricsCache(*config.CacheTTL),
		logger:                     logger,
		interval:                   *config.Interval,
		timeout:                    *config.Timeout,
	}
	return exporter
}

func (e *Route53Exporter) getRecordsPerHostedZoneMetrics(client awsclient.Client, hostedZones []*route53.HostedZone, ctx context.Context) []error {
	errChan := make(chan error, len(hostedZones))
	errs := []error{}

	wg := &sync.WaitGroup{}
	wg.Add(len(hostedZones))
	sem := make(chan int, route53MaxConcurrency)
	defer close(sem)
	for i, hostedZone := range hostedZones {

		sem <- 1
		go func(i int, hostedZone *route53.HostedZone) {
			defer func() {
				<-sem
				wg.Done()
			}()
			hostedZoneLimitOut, err := GetHostedZoneLimitWithBackoff(client, ctx, hostedZone.Id, maxRetries, e.logger)

			if err != nil {
				errChan <- fmt.Errorf("Could not get Limits for hosted zone with ID '%s' and name '%s'. Error was: %s", *hostedZone.Id, *hostedZone.Name, err.Error())
				awsclient.AwsExporterMetrics.IncrementErrors()
				return
			}
			e.logger.Info("msg", fmt.Sprintf("Currently at hosted zone: %d / %d", i, len(hostedZones)))
			e.cache.AddMetric(prometheus.MustNewConstMetric(e.RecordsPerHostedZoneQuota, prometheus.GaugeValue, float64(*hostedZoneLimitOut.Limit.Value), *hostedZone.Id, *hostedZone.Name))
			e.cache.AddMetric(prometheus.MustNewConstMetric(e.RecordsPerHostedZoneUsage, prometheus.GaugeValue, float64(*hostedZoneLimitOut.Count), *hostedZone.Id, *hostedZone.Name))

		}(i, hostedZone)
	}
	wg.Wait()
	close(errChan)

	for err := range errChan {
		errs = append(errs, err)
	}

	return errs
}

func (e *Route53Exporter) getHostedZonesPerAccountMetrics(client awsclient.Client, hostedZones []*route53.HostedZone, ctx context.Context) error {
	quota, err := getQuotaValueWithContext(client, route53ServiceCode, hostedZonesQuotaCode, ctx)
	if err != nil {
		return err
	}

	e.cache.AddMetric(prometheus.MustNewConstMetric(e.HostedZonesPerAccountQuota, prometheus.GaugeValue, quota))
	e.cache.AddMetric(prometheus.MustNewConstMetric(e.HostedZonesPerAccountUsage, prometheus.GaugeValue, float64(len(hostedZones))))
	return nil
}

// CollectLoop runs indefinitely to collect the route53 metrics in a cache. Metrics are only written into the cache once all have been collected to ensure that we don't have a partial collect.
func (e *Route53Exporter) CollectLoop() {
	client := awsclient.NewClientFromSession(e.sess)

	for {
		ctx, ctxCancelFunc := context.WithTimeout(context.Background(), e.timeout)
		e.Cancel = ctxCancelFunc
		e.logger.Info("msg", "Updating Route53 metrics...")

		hostedZones, err := getAllHostedZones(client, ctx, e.logger)

		e.logger.Info("msg", "Got all zones")
		if err != nil {
			e.logger.Error("msg", "Could not retrieve the list of hosted zones", "error", err.Error())
			awsclient.AwsExporterMetrics.IncrementErrors()
		}

		err = e.getHostedZonesPerAccountMetrics(client, hostedZones, ctx)
		if err != nil {
			e.logger.Error("msg", "Could not get limits for hosted zone", "error", err.Error())
			awsclient.AwsExporterMetrics.IncrementErrors()
		}

		errs := e.getRecordsPerHostedZoneMetrics(client, hostedZones, ctx)
		for _, err = range errs {
			e.logger.Error("msg", "Could not get limits for hosted zone", "error", err.Error())
			awsclient.AwsExporterMetrics.IncrementErrors()
		}

		e.logger.Info("msg", "Route53 metrics Updated")

		ctxCancelFunc() // should never do anything as we don't run stuff in the background

		time.Sleep(e.interval)
	}
}

func (e *Route53Exporter) Collect(ch chan<- prometheus.Metric) {
	for _, m := range e.cache.GetAllMetrics() {
		ch <- m
	}
}

func (e *Route53Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.RecordsPerHostedZoneQuota
	ch <- e.RecordsPerHostedZoneUsage
	ch <- e.LastUpdateTime
}

func getAllHostedZones(client awsclient.Client, ctx context.Context, logger *slog.Logger) ([]*route53.HostedZone, error) {
	result := []*route53.HostedZone{}

	listZonesInput := route53.ListHostedZonesInput{}

	listZonesOut, err := ListHostedZonesWithBackoff(client, ctx, &listZonesInput, maxRetries, logger)
	if err != nil {
		return nil, err
	}
	result = append(result, listZonesOut.HostedZones...)

	for *listZonesOut.IsTruncated {
		listZonesInput.Marker = listZonesOut.NextMarker
		listZonesOut, err = ListHostedZonesWithBackoff(client, ctx, &listZonesInput, maxRetries, logger)
		if err != nil {
			return nil, err
		}
		result = append(result, listZonesOut.HostedZones...)
	}

	return result, nil
}

func ListHostedZonesWithBackoff(client awsclient.Client, ctx context.Context, input *route53.ListHostedZonesInput, maxTries int, logger *slog.Logger) (*route53.ListHostedZonesOutput, error) {
	var listHostedZonesOut *route53.ListHostedZonesOutput
	var err error

	for i := 0; i < maxTries; i++ {
		listHostedZonesOut, err = client.ListHostedZonesWithContext(ctx, input)
		if err == nil {
			return listHostedZonesOut, err
		}
		if !isThrottlingError(err) {
			return nil, err
		}
		logger.Debug("msg", "Retrying throttling api call", "tries", i+1, "endpoint", "ListHostedZones")
		backOffSeconds := math.Pow(2, float64(i-1))
		time.Sleep(time.Duration(backOffSeconds) * time.Second)
	}
	return nil, err
}

func GetHostedZoneLimitWithBackoff(client awsclient.Client, ctx context.Context, hostedZoneId *string, maxTries int, logger *slog.Logger) (*route53.GetHostedZoneLimitOutput, error) {
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

		if !isThrottlingError(err) {
			return nil, err
		}
		logger.Debug("msg", "Retrying throttling api call", "tries", i+1, "endpoint", "GetHostedZoneLimit", "hostedZoneID", hostedZoneId)
		backOffSeconds := math.Pow(2, float64(i-1))
		time.Sleep(time.Duration(backOffSeconds) * time.Second)

	}
	return nil, err
}

func createGetHostedZoneLimitInput(hostedZoneId, limitType string) *route53.GetHostedZoneLimitInput {
	return &route53.GetHostedZoneLimitInput{
		HostedZoneId: aws.String(hostedZoneId),
		Type:         aws.String(limitType),
	}
}

func createListHostedZonesWithContext(maxItems string) *route53.ListHostedZonesInput {
	return &route53.ListHostedZonesInput{
		MaxItems: aws.String(maxItems),
	}
}

func createGetHostedZoneLimitWithContext(hostedZoneId, limitType string) *route53.GetHostedZoneLimitInput {
	return &route53.GetHostedZoneLimitInput{
		HostedZoneId: aws.String(hostedZoneId),
		Type:         aws.String(limitType),
	}
}

func getHostedZoneValueWithContext(client awsclient.Client, hostedZoneId string, limitType string, ctx context.Context) (int64, error) {
	sqOutput, err := client.GetHostedZoneLimitWithContext(ctx, createGetHostedZoneLimitInput(hostedZoneId, limitType))

	if err != nil {
		return 0, err
	}

	return *sqOutput.Limit.Value, nil
}

// isThrottlingError returns true if the error given is an instance of awserr.Error and the error code matches the constant errorCodeThrottling. It's not compared against route53.ErrCodeThrottlingException as this does not match what the api is returning.
func isThrottlingError(err error) bool {
	awsError, isAwsError := err.(awserr.Error)
	return isAwsError && awsError.Code() == errorCodeThrottling
}
