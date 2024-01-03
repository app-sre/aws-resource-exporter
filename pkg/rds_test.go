package pkg

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/app-sre/aws-resource-exporter/pkg/awsclient"
	"github.com/app-sre/aws-resource-exporter/pkg/awsclient/mock"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/go-kit/kit/log"
	"github.com/golang/mock/gomock"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
)

func createTestDBInstances() []*rds.DBInstance {
	return []*rds.DBInstance{
		{
			DBInstanceIdentifier: aws.String("footest"),
			DBInstanceClass:      aws.String("db.m5.xlarge"),
			DBParameterGroups:    []*rds.DBParameterGroupStatus{{DBParameterGroupName: aws.String("default.postgres14")}},
			PubliclyAccessible:   aws.Bool(false),
			StorageEncrypted:     aws.Bool(false),
			AllocatedStorage:     aws.Int64(1024),
			DBInstanceStatus:     aws.String("on fire"),
			Engine:               aws.String("SQL"),
			EngineVersion:        aws.String("1000"),
		},
	}
}

func TestRequestRDSLogMetrics(t *testing.T) {
	ctx := context.TODO()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)
	mockClient.EXPECT().DescribeDBLogFilesAll(ctx, "footest").Return([]*rds.DescribeDBLogFilesOutput{
		{DescribeDBLogFiles: []*rds.DescribeDBLogFilesDetails{{Size: aws.Int64(123)}, {Size: aws.Int64(123)}}},
		{DescribeDBLogFiles: []*rds.DescribeDBLogFilesDetails{{Size: aws.Int64(1)}}},
	}, nil)

	x := RDSExporter{
		svcs: []awsclient.Client{mockClient},
	}

	metrics, err := x.requestRDSLogMetrics(ctx, 0, "footest")
	assert.Equal(t, int64(247), metrics.totalLogSize)
	assert.Equal(t, 3, metrics.logs)
	assert.Nil(t, err)
}

func TestAddRDSLogMetrics(t *testing.T) {
	ctx := context.TODO()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)
	mockClient.EXPECT().DescribeDBLogFilesAll(ctx, "footest").Return([]*rds.DescribeDBLogFilesOutput{
		{DescribeDBLogFiles: []*rds.DescribeDBLogFilesDetails{{Size: aws.Int64(123)}, {Size: aws.Int64(123)}}},
		{DescribeDBLogFiles: []*rds.DescribeDBLogFilesDetails{{Size: aws.Int64(1)}}},
	}, nil)

	x := RDSExporter{
		svcs:     []awsclient.Client{mockClient},
		sessions: []*session.Session{session.New(&aws.Config{Region: aws.String("foo")})},
		cache:    *NewMetricsCache(10 * time.Second),
	}

	err := x.addRDSLogMetrics(ctx, 0, "footest")
	assert.Len(t, x.cache.GetAllMetrics(), 2)
	assert.Nil(t, err)
}

func TestAddAllInstanceMetrics(t *testing.T) {
	x := RDSExporter{
		sessions: []*session.Session{session.New(&aws.Config{Region: aws.String("foo")})},
		cache:    *NewMetricsCache(10 * time.Second),
		logger:   log.NewNopLogger(),
	}

	var instances = []*rds.DBInstance{}

	// Test with no match
	eolInfos := []EOLInfo{
		{Engine: "engine", Version: "123", EOL: "2023-12-01"},
	}

	x.addAllInstanceMetrics(0, instances, eolInfos)
	assert.Len(t, x.cache.GetAllMetrics(), 0)

	x.addAllInstanceMetrics(0, createTestDBInstances(), eolInfos)
	assert.Len(t, x.cache.GetAllMetrics(), 9)
}

func TestAddAllInstanceMetricsWithEOLMatch(t *testing.T) {
	thresholds := []Threshold{
		{Name: "red", Threshold: 90},
		{Name: "yellow", Threshold: 180},
		{Name: "green", Threshold: 365},
	}

	x := RDSExporter{
		sessions:   []*session.Session{session.New(&aws.Config{Region: aws.String("foo")})},
		cache:      *NewMetricsCache(10 * time.Second),
		logger:     log.NewNopLogger(),
		thresholds: thresholds,
	}

	eolInfos := []EOLInfo{
		{Engine: "SQL", Version: "1000", EOL: "2000-12-01"},
	}

	x.addAllInstanceMetrics(0, createTestDBInstances(), eolInfos)

	labels, err := getMetricLabels(&x, EOLInfos, "eol_date", "eol_status")
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

func TestAddAllInstanceMetricsWithGetEOLStatusError(t *testing.T) {
	x := RDSExporter{
		sessions: []*session.Session{session.New(&aws.Config{Region: aws.String("foo")})},
		cache:    *NewMetricsCache(10 * time.Second),
		logger:   log.NewNopLogger(),
	}

	eolInfos := []EOLInfo{
		{Engine: "SQL", Version: "1000", EOL: "invalid-date"},
	}

	x.addAllInstanceMetrics(0, createTestDBInstances(), eolInfos)

	labels, err := getMetricLabels(&x, EOLInfos, "eol_date", "eol_status")

	if err == nil {
		t.Errorf("Expected an error from getMetricLabels but got none")
	}
	if len(labels) > 0 {
		t.Errorf("Expected no labels to be returned, got: %v", labels)
	}
}

func TestGetEOLStatus(t *testing.T) {
	x := RDSExporter{
		sessions: []*session.Session{session.New(&aws.Config{Region: aws.String("foo")})},
		cache:    *NewMetricsCache(10 * time.Second),
		logger:   log.NewNopLogger(),
	}

	thresholds := []Threshold{
		{Name: "red", Threshold: 90},
		{Name: "yellow", Threshold: 180},
		{Name: "green", Threshold: 365},
	}

	// EOL date is within 90 days
	eol := time.Now().Add(2 * 24 * time.Hour).Format("2006-01-02")
	expectedStatus := "red"
	status, err := x.getEOLStatus(eol, thresholds)
	if err != nil {
		t.Errorf("Expected no error, but got an error: %v", err)
	}
	if status != expectedStatus {
		t.Errorf("Expected status '%s', but got '%s'", expectedStatus, status)
	}

	// EOL date is within 180 days
	eol = time.Now().Add(120 * 24 * time.Hour).Format("2006-01-02")
	expectedStatus = "yellow"
	status, err = x.getEOLStatus(eol, thresholds)
	if err != nil {
		t.Errorf("Expected no error, but got an error: %v", err)
	}
	if status != expectedStatus {
		t.Errorf("Expected status '%s', but got '%s'", expectedStatus, status)
	}

	// EOL date is more than 180 days
	eol = time.Now().Add(200 * 24 * time.Hour).Format("2006-01-02")
	expectedStatus = "green"
	status, err = x.getEOLStatus(eol, thresholds)
	if err != nil {
		t.Errorf("Expected no error, but got an error: %v", err)
	}
	if status != expectedStatus {
		t.Errorf("Expected status '%s', but got '%s'", expectedStatus, status)
	}

}

func TestEngineVersionMetricIncludesAWSAccountId(t *testing.T) {
	x := RDSExporter{
		sessions:     []*session.Session{session.New(&aws.Config{Region: aws.String("foo")})},
		cache:        *NewMetricsCache(10 * time.Second),
		logger:       log.NewNopLogger(),
		awsAccountId: "1234567890",
	}

	x.addAllInstanceMetrics(0, createTestDBInstances(), nil)

	labels, err := getMetricLabels(&x, EngineVersion, "aws_account_id")
	if err != nil {
		t.Fatalf("Failed to get metric labels: %v", err)
	}

	if accountId, ok := labels["aws_account_id"]; !ok || accountId != "1234567890" {
		t.Errorf("aws_account_id label has an unexpected value. Expected: %s, Actual: %s", "1234567890", accountId)
	}
}

// Helper function to retrieve metric values from the cache
func getMetricLabels(x *RDSExporter, metricDesc *prometheus.Desc, labelNames ...string) (map[string]string, error) {
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

func TestAddAllPendingMaintenancesMetrics(t *testing.T) {
	ctx := context.TODO()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)
	mockClient.EXPECT().DescribePendingMaintenanceActionsAll(ctx).Return([]*rds.ResourcePendingMaintenanceActions{
		{
			PendingMaintenanceActionDetails: []*rds.PendingMaintenanceAction{{
				Action:      aws.String("something going on"),
				Description: aws.String("plumbing"),
			}},
			ResourceIdentifier: aws.String("::::::footest"),
		},
	}, nil)

	x := RDSExporter{
		svcs:     []awsclient.Client{mockClient},
		sessions: []*session.Session{session.New(&aws.Config{Region: aws.String("foo")})},
		cache:    *NewMetricsCache(10 * time.Second),
		logger:   log.NewNopLogger(),
	}

	x.addAllPendingMaintenancesMetrics(ctx, 0, createTestDBInstances())
	metrics := x.cache.GetAllMetrics()
	assert.Len(t, metrics, 1)

	var dto dto.Metric
	metrics[0].Write(&dto)

	// Expecting a maintenance, thus value 1
	assert.Equal(t, float64(1), *dto.Gauge.Value)

}

func TestAddAllPendingMaintenancesNoMetrics(t *testing.T) {
	ctx := context.TODO()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)
	mockClient.EXPECT().DescribePendingMaintenanceActionsAll(ctx).Return([]*rds.ResourcePendingMaintenanceActions{}, nil)

	x := RDSExporter{
		svcs:     []awsclient.Client{mockClient},
		sessions: []*session.Session{session.New(&aws.Config{Region: aws.String("foo")})},
		cache:    *NewMetricsCache(10 * time.Second),
		logger:   log.NewNopLogger(),
	}

	x.addAllPendingMaintenancesMetrics(ctx, 0, createTestDBInstances())
	metrics := x.cache.GetAllMetrics()
	assert.Len(t, metrics, 1)

	var dto dto.Metric
	metrics[0].Write(&dto)

	// Expecting no maintenance, thus 0 value
	assert.Equal(t, float64(0), *dto.Gauge.Value)
}
