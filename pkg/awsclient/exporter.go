package awsclient

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var AwsExporterMetrics *ExporterMetrics

// ExporterMetrics defines an instance of the exporter metrics
type ExporterMetrics struct {
	Requests prometheus.Counter
	Errors   prometheus.Counter
	mutex    *sync.Mutex
}

// NewExporterMetrics creates a new exporter metrics instance
func NewExporterMetrics(namespace string) *ExporterMetrics {
	return &ExporterMetrics{
		Requests: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "aws",
			Name:      "requests_total",
			Help:      "The total number of AWS API requests.",
		}),
		Errors: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "aws",
			Name:      "errors_total",
			Help:      "The total number of errors encountered by the exporter.",
		}),
		mutex: &sync.Mutex{},
	}
}

// Describe is used by the Prometheus client to return a description of the metrics
func (e *ExporterMetrics) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.Requests.Desc()
	ch <- e.Errors.Desc()
}

// Collect is used by the Prometheus client to collect and return the metrics values
func (e *ExporterMetrics) Collect(ch chan<- prometheus.Metric) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	ch <- e.Requests
	ch <- e.Errors
}

// IncrementRequests is used to increment the APIRequestsCount
func (e *ExporterMetrics) IncrementRequests() {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.Requests.Inc()
}

// IncrementErrors is used to increment the APIErrorsCount
func (e *ExporterMetrics) IncrementErrors() {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.Errors.Inc()
}
