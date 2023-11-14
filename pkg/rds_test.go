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
	x := RDSExporter{
		sessions: []*session.Session{session.New(&aws.Config{Region: aws.String("foo")})},
		cache:    *NewMetricsCache(10 * time.Second),
		logger:   log.NewNopLogger(),
	}

	eolInfos := []EOLInfo{
		{Engine: "SQL", Version: "1000", EOL: "2023-12-01"},
	}

	x.addAllInstanceMetrics(0, createTestDBInstances(), eolInfos)

	eolDateMetricValue, err := getEOLDateMetricValue(&x)
	if err != nil {
		t.Errorf("Error retrieving EOLDate metric value: %v", err)
	}

	expectedEOLDate := "2023-12-01"

	if eolDateMetricValue != expectedEOLDate {
		t.Errorf("EOLDate metric has an unexpected value. Expected: %s, Actual: %s", expectedEOLDate, eolDateMetricValue)
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

	_, err := getEOLStatusMetricValue(&x)
	if err == nil {
		t.Error("Expected an error from getEOLStatusMetricValue but got none")
	} else {
		t.Logf("Expected error received from getEOLStatusMetricValue: %v", err)
	}
}

// Helper function to retrieve the EOLDate metric value from the cache
func getEOLDateMetricValue(x *RDSExporter) (string, error) {
	metrics := x.cache.GetAllMetrics()
	metricDescription := EOLDate.String()

	for _, metric := range metrics {
		if metric.Desc().String() == metricDescription {
			dto := &dto.Metric{}
			if err := metric.Write(dto); err != nil {
				return "", err
			}
			for _, label := range dto.GetLabel() {
				if label.GetName() == "eol_date" {
					return label.GetValue(), nil
				}
			}
			return "", fmt.Errorf("eol_date label not found for EOLDate metric")
		}
	}
	return "", fmt.Errorf("EOLDate metric not found")
}

// Helper function to retrieve the EOLStatus metric value from the cache
func getEOLStatusMetricValue(x *RDSExporter) (string, error) {
	metrics := x.cache.GetAllMetrics()
	metricDescription := EOLStatus.String()

	for _, metric := range metrics {
		if metric.Desc().String() == metricDescription {
			dto := &dto.Metric{}
			if err := metric.Write(dto); err != nil {
				return "", err
			}
			for _, label := range dto.GetLabel() {
				if label.GetName() == "eol_status" {
					return label.GetValue(), nil
				}
			}
			return "", fmt.Errorf("eol_status label not found for EOLStatus metric")
		}
	}
	return "", fmt.Errorf("EOLStatus metric not found")
}

func TestGetEOLStatus(t *testing.T) {
	// EOL date is within 90 days
	eol := time.Now().Add(2 * 24 * time.Hour).Format("2006-01-02")
	expectedStatus := "red"
	status, err := getEOLStatus(eol)
	if err != nil {
		t.Errorf("Expected no error, but got an error: %v", err)
	}
	if status != expectedStatus {
		t.Errorf("Expected status '%s', but got '%s'", expectedStatus, status)
	}

	// EOL date is within 180 days
	eol = time.Now().Add(120 * 24 * time.Hour).Format("2006-01-02")
	expectedStatus = "yellow"
	status, err = getEOLStatus(eol)
	if err != nil {
		t.Errorf("Expected no error, but got an error: %v", err)
	}
	if status != expectedStatus {
		t.Errorf("Expected status '%s', but got '%s'", expectedStatus, status)
	}

	// EOL date is more than 180 days
	eol = time.Now().Add(200 * 24 * time.Hour).Format("2006-01-02")
	expectedStatus = "green"
	status, err = getEOLStatus(eol)
	if err != nil {
		t.Errorf("Expected no error, but got an error: %v", err)
	}
	if status != expectedStatus {
		t.Errorf("Expected status '%s', but got '%s'", expectedStatus, status)
	}

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
