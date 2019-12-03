package main

import (
	"sync"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/prometheus/client_golang/prometheus"
)

// ExporterMetrics defines an instance of the exporter metrics
type ExporterMetrics struct {
	sess *session.Session

	APIRequestsCount float64
	APIErrorsCount   float64

	APIRequests *prometheus.Desc
	APIErrors   *prometheus.Desc

	mutex *sync.Mutex
}

// NewExporterMetrics creates a new exporter metrics instance
func NewExporterMetrics(sess *session.Session) *ExporterMetrics {
	return &ExporterMetrics{
		sess: sess,
		APIRequests: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "apirequests"),
			"API requests made by the exporter.",
			[]string{},
			nil,
		),
		APIErrors: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "apierrors"),
			"API errors encountered by the exporter.",
			[]string{},
			nil,
		),
		mutex: &sync.Mutex{},
	}
}

// Describe is used by the Prometheus client to return a description of the metrics
func (e *ExporterMetrics) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.APIRequests
	ch <- e.APIErrors
}

// Collect is used by the Prometheus client to collect and return the metrics values
func (e *ExporterMetrics) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(e.APIRequests, prometheus.CounterValue, e.APIRequestsCount)
	ch <- prometheus.MustNewConstMetric(e.APIErrors, prometheus.CounterValue, e.APIErrorsCount)
}

// IncrementRequests increments the API requests counter
func (e *ExporterMetrics) IncrementRequests() {
	e.mutex.Lock()
	e.APIRequestsCount++
	e.mutex.Unlock()
}

// IncrementErrors increments the API requests error counter
func (e *ExporterMetrics) IncrementErrors() {
	e.mutex.Lock()
	e.APIErrorsCount++
	e.mutex.Unlock()
}
