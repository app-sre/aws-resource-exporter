package pkg

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kafka"
	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func createTestClusters() []*kafka.ClusterInfo {
	return []*kafka.ClusterInfo{
		{
			ClusterName: aws.String("test-cluster-1"),
			CurrentBrokerSoftwareInfo: &kafka.BrokerSoftwareInfo{
				KafkaVersion: aws.String("1000"),
			},
		},
	}
}

func TestAddAllMSKMetricsWithEOLMatch(t *testing.T) {
	thresholds := []Threshold{
		{Name: "red", Days: 90},
		{Name: "yellow", Days: 180},
		{Name: "green", Days: 365},
	}

	e := MSKExporter{
		sessions:   []*session.Session{session.New(&aws.Config{Region: aws.String("foo")})},
		cache:      *NewMetricsCache(10 * time.Second),
		logger:     log.NewNopLogger(),
		thresholds: thresholds,
	}

	mskInfos := []MSKInfo{
		{Version: "1000", EOL: "2000-12-01"},
	}

	e.addMetricFromMSKInfo(0, createTestClusters(), mskInfos)

	labels, err := getMSKMetricLabels(&e, MSKInfos, "eol_date", "eol_status")
	if err != nil {
		t.Errorf("Error retrieving EOL labels: %v", err)
	}

	expectedEOLDate := "2000-12-01"
	expectedEOLStatus := "red"

	if eolDate, ok := labels["eol_date"]; !ok || eolDate != expectedEOLDate {
		t.Errorf("EOLDate metric has an unexpected value. Expected: %s, Actual: %s", expectedEOLDate, eolDate)
	}

	if eolStatus, ok := labels["eol_status"]; !ok || eolStatus != expectedEOLStatus {
		t.Errorf("EOLStatus metric has an unexpected value. Expected: %s, Actual: %s", expectedEOLStatus, eolStatus)
	}
}

func getMSKMetricLabels(x *MSKExporter, metricDesc *prometheus.Desc, labelNames ...string) (map[string]string, error) {
	metricDescription := metricDesc.String()
	metrics := x.cache.GetAllMetrics()

	for _, metric := range metrics {
		if metric.Desc().String() == metricDescription {
			dtoMetric := &dto.Metric{}
			if err := metric.Write(dtoMetric); err != nil {
				return nil, err
			}

			labelValues := make(map[string]string)
			for _, label := range dtoMetric.GetLabel() {
				for _, labelName := range labelNames {
					if label.GetName() == labelName {
						labelValues[labelName] = label.GetValue()
					}
				}
			}

			if len(labelValues) != len(labelNames) {
				return nil, fmt.Errorf("not all requested labels found in metric")
			}

			return labelValues, nil
		}
	}
	return nil, fmt.Errorf("metric not found")
}
