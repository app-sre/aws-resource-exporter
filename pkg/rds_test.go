package pkg

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/app-sre/aws-resource-exporter/pkg/awsclient"
	"github.com/app-sre/aws-resource-exporter/pkg/awsclient/mock"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rds_types "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/golang/mock/gomock"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
)

// TestMain initializes awsclient.AwsExporterMetrics, which production code only sets up in
// main.go. Several error paths under test (e.g. resolveRDSEOLDate) call
// awsclient.AwsExporterMetrics.IncrementErrors() and would otherwise nil-panic.
func TestMain(m *testing.M) {
	awsclient.AwsExporterMetrics = awsclient.NewExporterMetrics("test")
	os.Exit(m.Run())
}

func createTestDBInstances() []rds_types.DBInstance {
	return []rds_types.DBInstance{
		{
			DBInstanceIdentifier: aws.String("footest"),
			DBInstanceClass:      aws.String("db.m5.xlarge"),
			DBParameterGroups:    []rds_types.DBParameterGroupStatus{{DBParameterGroupName: aws.String("default.postgres14")}},
			PubliclyAccessible:   aws.Bool(false),
			StorageEncrypted:     aws.Bool(false),
			AllocatedStorage:     aws.Int32(1024),
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
		{DescribeDBLogFiles: []rds_types.DescribeDBLogFilesDetails{{Size: aws.Int64(123)}, {Size: aws.Int64(123)}}},
		{DescribeDBLogFiles: []rds_types.DescribeDBLogFilesDetails{{Size: aws.Int64(1)}}},
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
		{DescribeDBLogFiles: []rds_types.DescribeDBLogFilesDetails{{Size: aws.Int64(123)}, {Size: aws.Int64(123)}}},
		{DescribeDBLogFiles: []rds_types.DescribeDBLogFilesDetails{{Size: aws.Int64(1)}}},
	}, nil)

	x := RDSExporter{
		svcs:    []awsclient.Client{mockClient},
		configs: []aws.Config{{Region: "foo"}},
		cache:   *NewMetricsCache(10 * time.Second),
	}

	err := x.addRDSLogMetrics(ctx, 0, "footest")
	assert.Len(t, x.cache.GetAllMetrics(), 2)
	assert.Nil(t, err)
}

// resetRDSEOLCache clears the package-level metricsProxy cache used by resolveRDSEOLDate(s).
// It must be called at the start of any test that exercises the live EOL lookup, since
// the cache is a process-global shared across tests.
func resetRDSEOLCache() {
	metricsProxy = NewMetricProxy()
}

func TestAddAllInstanceMetrics(t *testing.T) {
	resetRDSEOLCache()
	ctx := context.TODO()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)
	mockClient.EXPECT().DescribeDBEngineVersion(ctx, "SQL", "1000").Return(nil, fmt.Errorf("engine not found"))

	x := RDSExporter{
		svcs:    []awsclient.Client{mockClient},
		configs: []aws.Config{{Region: "foo"}},
		cache:   *NewMetricsCache(10 * time.Second),
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	var instances = []rds_types.DBInstance{}

	x.addAllInstanceMetrics(ctx, 0, instances)
	assert.Len(t, x.cache.GetAllMetrics(), 0)

	x.addAllInstanceMetrics(ctx, 0, createTestDBInstances())
	assert.Len(t, x.cache.GetAllMetrics(), 10)
}

func TestAddAllInstanceMetricsWithEOLMiss(t *testing.T) {
	resetRDSEOLCache()
	ctx := context.TODO()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// AWS has no lifecycle data for this engine/version (e.g. a non-open-source engine)
	mockClient := mock.NewMockClient(ctrl)
	mockClient.EXPECT().DescribeDBEngineVersion(ctx, "SQL", "1000").Return(nil, fmt.Errorf("engine not found"))

	x := RDSExporter{
		svcs:    []awsclient.Client{mockClient},
		configs: []aws.Config{{Region: "foo"}},
		cache:   *NewMetricsCache(10 * time.Second),
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	x.addAllInstanceMetrics(ctx, 0, createTestDBInstances())

	labels, err := getMetricLabels(&x, EOLInfos, "eol_date", "eol_status")
	if err != nil {
		t.Errorf("Error retrieving EOL labels: %v", err)
	}

	expectedEOLDate := "no-eol-date"
	expectedEOLStatus := "red"

	if eolDate, ok := labels["eol_date"]; !ok || eolDate != expectedEOLDate {
		t.Errorf("EOLDate metric has an unexpected value. Expected: %s, Actual: %s", expectedEOLDate, eolDate)
	}

	if eolStatus, ok := labels["eol_status"]; !ok || eolStatus != expectedEOLStatus {
		t.Errorf("EOLStatus metric has an unexpected value. Expected: %s, Actual: %s", expectedEOLStatus, eolStatus)
	}
}

func TestAddAllInstanceMetricsWithEOLMatch(t *testing.T) {
	resetRDSEOLCache()
	thresholds := []Threshold{
		{Name: "red", Days: 90},
		{Name: "yellow", Days: 180},
		{Name: "green", Days: 365},
	}

	expectedEOLDate := "2000-12-01"
	eolDateTime, err := time.Parse("2006-01-02", expectedEOLDate)
	assert.NoError(t, err)

	ctx := context.TODO()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)
	mockClient.EXPECT().DescribeDBEngineVersion(ctx, "SQL", "1000").Return(&rds_types.DBEngineVersion{
		MajorEngineVersion: aws.String("1000"),
	}, nil)
	mockClient.EXPECT().DescribeDBMajorEngineVersion(ctx, "SQL", "1000").Return(&rds_types.DBMajorEngineVersion{
		SupportedEngineLifecycles: []rds_types.SupportedEngineLifecycle{
			{
				LifecycleSupportName:    rds_types.LifecycleSupportNameOpenSourceRdsStandardSupport,
				LifecycleSupportEndDate: aws.Time(eolDateTime),
			},
		},
	}, nil)

	x := RDSExporter{
		svcs:       []awsclient.Client{mockClient},
		configs:    []aws.Config{{Region: "foo"}},
		cache:      *NewMetricsCache(10 * time.Second),
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		thresholds: thresholds,
	}

	x.addAllInstanceMetrics(ctx, 0, createTestDBInstances())

	labels, err := getMetricLabels(&x, EOLInfos, "eol_date", "eol_status")
	if err != nil {
		t.Errorf("Error retrieving EOL labels: %v", err)
	}

	expectedEOLStatus := "red"

	if eolDate, ok := labels["eol_date"]; !ok || eolDate != expectedEOLDate {
		t.Errorf("EOLDate metric has an unexpected value. Expected: %s, Actual: %s", expectedEOLDate, eolDate)
	}

	if eolStatus, ok := labels["eol_status"]; !ok || eolStatus != expectedEOLStatus {
		t.Errorf("EOLStatus metric has an unexpected value. Expected: %s, Actual: %s", expectedEOLStatus, eolStatus)
	}
}

// A misconfigured exporter (no thresholds) successfully resolves an EOL date but can't turn it
// into a status. addAllInstanceMetrics silently drops the EOLInfos metric for that instance in
// this case rather than falling back to no-eol-date/red -- this test documents and locks in
// that (perhaps surprising) behavior.
func TestAddAllInstanceMetricsSilentlyDropsMetricOnGetEOLStatusError(t *testing.T) {
	resetRDSEOLCache()
	eolDateTime, err := time.Parse("2006-01-02", "2000-12-01")
	assert.NoError(t, err)

	ctx := context.TODO()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)
	mockClient.EXPECT().DescribeDBEngineVersion(ctx, "SQL", "1000").Return(&rds_types.DBEngineVersion{
		MajorEngineVersion: aws.String("1000"),
	}, nil)
	mockClient.EXPECT().DescribeDBMajorEngineVersion(ctx, "SQL", "1000").Return(&rds_types.DBMajorEngineVersion{
		SupportedEngineLifecycles: []rds_types.SupportedEngineLifecycle{
			{
				LifecycleSupportName:    rds_types.LifecycleSupportNameOpenSourceRdsStandardSupport,
				LifecycleSupportEndDate: aws.Time(eolDateTime),
			},
		},
	}, nil)

	x := RDSExporter{
		svcs:       []awsclient.Client{mockClient},
		configs:    []aws.Config{{Region: "foo"}},
		cache:      *NewMetricsCache(10 * time.Second),
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		thresholds: []Threshold{}, // empty thresholds makes GetEOLStatus return an error
	}

	x.addAllInstanceMetrics(ctx, 0, createTestDBInstances())

	_, err = getMetricLabels(&x, EOLInfos, "eol_date", "eol_status")
	if err == nil {
		t.Errorf("Expected the EOLInfos metric to be dropped when GetEOLStatus errors, but it was found")
	}
}

func TestResolveRDSEOLDatesCachesResult(t *testing.T) {
	resetRDSEOLCache()
	ctx := context.TODO()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eolDateTime, err := time.Parse("2006-01-02", "2000-12-01")
	assert.NoError(t, err)

	mockClient := mock.NewMockClient(ctrl)
	// Expect exactly one call each despite resolving the same instance's engine/version twice below
	mockClient.EXPECT().DescribeDBEngineVersion(ctx, "SQL", "1000").Return(&rds_types.DBEngineVersion{
		MajorEngineVersion: aws.String("1000"),
	}, nil).Times(1)
	mockClient.EXPECT().DescribeDBMajorEngineVersion(ctx, "SQL", "1000").Return(&rds_types.DBMajorEngineVersion{
		SupportedEngineLifecycles: []rds_types.SupportedEngineLifecycle{
			{
				LifecycleSupportName:    rds_types.LifecycleSupportNameOpenSourceRdsStandardSupport,
				LifecycleSupportEndDate: aws.Time(eolDateTime),
			},
		},
	}, nil).Times(1)

	x := RDSExporter{
		svcs:    []awsclient.Client{mockClient},
		configs: []aws.Config{{Region: "foo"}},
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		workers: 5,
	}

	instances := createTestDBInstances()

	x.resolveRDSEOLDates(ctx, 0, instances)
	eolDate, ok := rdsEOLCacheLookup("SQL", "1000")
	assert.True(t, ok)
	assert.Equal(t, "2000-12-01", eolDate)

	// Resolving the same engine/version again must be served from cache, not AWS
	x.resolveRDSEOLDates(ctx, 0, instances)
	eolDate, ok = rdsEOLCacheLookup("SQL", "1000")
	assert.True(t, ok)
	assert.Equal(t, "2000-12-01", eolDate)
}

func TestResolveRDSEOLDatesCachesAPIErrorsWithShortTTL(t *testing.T) {
	resetRDSEOLCache()
	ctx := context.TODO()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)
	// A failed AWS call (e.g. missing IAM permissions) must still be cached, just with a much
	// shorter TTL than a confirmed answer -- otherwise, with a scrape interval as short as 15s
	// by default, a persistent failure would retry on every single scrape forever.
	mockClient.EXPECT().DescribeDBEngineVersion(ctx, "SQL", "1000").Return(nil, fmt.Errorf("throttled")).Times(1)

	x := RDSExporter{
		svcs:    []awsclient.Client{mockClient},
		configs: []aws.Config{{Region: "foo"}},
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		workers: 5,
	}

	instances := createTestDBInstances()

	x.resolveRDSEOLDates(ctx, 0, instances)
	// A failure is never a confirmed answer, so the shared engine/version cache stays empty...
	_, cached := rdsEOLCacheLookup("SQL", "1000")
	assert.False(t, cached)
	// ...but it is recorded in this region's error cache so it isn't retried immediately
	assert.True(t, rdsEOLRecentlyFailed("foo", "SQL", "1000"))

	// Resolving again immediately must be served from the error cache, not retried against AWS
	x.resolveRDSEOLDates(ctx, 0, instances)
	assert.True(t, rdsEOLRecentlyFailed("foo", "SQL", "1000"))

	item, err := metricsProxy.GetMetricById(rdsEOLErrorCacheKey("foo", "SQL", "1000"))
	assert.NoError(t, err)
	assert.Equal(t, rdsEOLErrorCacheTTLSeconds, item.ttl)
}

func TestResolveRDSEOLDatesFailureIsScopedToRegion(t *testing.T) {
	resetRDSEOLCache()
	ctx := context.TODO()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eolDateTime, err := time.Parse("2006-01-02", "2000-12-01")
	assert.NoError(t, err)

	// Region A's AWS call fails...
	mockClientA := mock.NewMockClient(ctrl)
	mockClientA.EXPECT().DescribeDBEngineVersion(ctx, "SQL", "1000").Return(nil, fmt.Errorf("throttled"))

	// ...but region B, same engine/version, must still make its own attempt and succeed
	mockClientB := mock.NewMockClient(ctrl)
	mockClientB.EXPECT().DescribeDBEngineVersion(ctx, "SQL", "1000").Return(&rds_types.DBEngineVersion{
		MajorEngineVersion: aws.String("1000"),
	}, nil)
	mockClientB.EXPECT().DescribeDBMajorEngineVersion(ctx, "SQL", "1000").Return(&rds_types.DBMajorEngineVersion{
		SupportedEngineLifecycles: []rds_types.SupportedEngineLifecycle{
			{
				LifecycleSupportName:    rds_types.LifecycleSupportNameOpenSourceRdsStandardSupport,
				LifecycleSupportEndDate: aws.Time(eolDateTime),
			},
		},
	}, nil)

	x := RDSExporter{
		svcs:    []awsclient.Client{mockClientA, mockClientB},
		configs: []aws.Config{{Region: "region-a"}, {Region: "region-b"}},
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		workers: 5,
	}

	instances := createTestDBInstances()

	x.resolveRDSEOLDates(ctx, 0, instances)
	_, cached := rdsEOLCacheLookup("SQL", "1000")
	assert.False(t, cached, "a failed lookup in one region must not be cached as a confirmed answer")

	x.resolveRDSEOLDates(ctx, 1, instances)
	eolDate, cached := rdsEOLCacheLookup("SQL", "1000")
	assert.True(t, cached, "region B must still attempt and resolve its own lookup despite region A's failure")
	assert.Equal(t, "2000-12-01", eolDate)
}

func TestResolveRDSEOLDateCachesPendingLifecycleEntryWithShortTTL(t *testing.T) {
	resetRDSEOLCache()
	ctx := context.TODO()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)
	mockClient.EXPECT().DescribeDBEngineVersion(ctx, "postgres", "18.0").Return(&rds_types.DBEngineVersion{
		MajorEngineVersion: aws.String("18"),
	}, nil)
	// Standard-support entry exists, but AWS hasn't published an end date yet (e.g. a
	// just-released minor version)
	mockClient.EXPECT().DescribeDBMajorEngineVersion(ctx, "postgres", "18").Return(&rds_types.DBMajorEngineVersion{
		SupportedEngineLifecycles: []rds_types.SupportedEngineLifecycle{
			{
				LifecycleSupportName:    rds_types.LifecycleSupportNameOpenSourceRdsStandardSupport,
				LifecycleSupportEndDate: nil,
			},
		},
	}, nil)

	x := RDSExporter{
		svcs:    []awsclient.Client{mockClient},
		configs: []aws.Config{{Region: "foo"}},
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	x.resolveRDSEOLDate(ctx, 0, "postgres", "18.0")

	eolDate, cached := rdsEOLCacheLookup("postgres", "18.0")
	assert.True(t, cached)
	assert.Equal(t, "", eolDate)

	item, err := metricsProxy.GetMetricById(rdsEOLCacheKey("postgres", "18.0"))
	assert.NoError(t, err)
	assert.Equal(t, rdsEOLPendingCacheTTLSeconds, item.ttl)
}

func TestGetEOLStatus(t *testing.T) {
	thresholds := []Threshold{
		{Name: "red", Days: 90},
		{Name: "yellow", Days: 180},
		{Name: "green", Days: 365},
	}

	// EOL date is within 90 days
	eol := time.Now().Add(2 * 24 * time.Hour).Format("2006-01-02")
	expectedStatus := "red"
	status, err := GetEOLStatus(eol, thresholds)
	if err != nil {
		t.Errorf("Expected no error, but got an error: %v", err)
	}
	if status != expectedStatus {
		t.Errorf("Expected status '%s', but got '%s'", expectedStatus, status)
	}

	// EOL date is within 180 days
	eol = time.Now().Add(120 * 24 * time.Hour).Format("2006-01-02")
	expectedStatus = "yellow"
	status, err = GetEOLStatus(eol, thresholds)
	if err != nil {
		t.Errorf("Expected no error, but got an error: %v", err)
	}
	if status != expectedStatus {
		t.Errorf("Expected status '%s', but got '%s'", expectedStatus, status)
	}

	// EOL date is more than 180 days
	eol = time.Now().Add(200 * 24 * time.Hour).Format("2006-01-02")
	expectedStatus = "green"
	status, err = GetEOLStatus(eol, thresholds)
	if err != nil {
		t.Errorf("Expected no error, but got an error: %v", err)
	}
	if status != expectedStatus {
		t.Errorf("Expected status '%s', but got '%s'", expectedStatus, status)
	}

	//EOL date exceeds highest threshold
	eol = time.Now().Add(400 * 24 * time.Hour).Format("2006-01-02")
	expectedStatus = "green"
	status, err = GetEOLStatus(eol, thresholds)
	if err != nil {
		t.Errorf("Expected no error, but got an error: %v", err)
	}
	if status != expectedStatus {
		t.Errorf("Expected status '%s', but got '%s'", expectedStatus, status)
	}

	//Thresholds is empty
	eol = time.Now().Add(30 * 24 * time.Hour).Format("2006-01-02")
	emptyThresholds := []Threshold{}
	status, err = GetEOLStatus(eol, emptyThresholds)
	if err == nil {
		t.Errorf("Expected an error for empty thresholds, but got none")
	}
	if status != "" {
		t.Errorf("Expected no status for empty thresholds, but got '%s'", status)
	}

	//EOL date is not a parseable date
	status, err = GetEOLStatus("invalid-date", thresholds)
	if err == nil {
		t.Errorf("Expected an error for an unparseable EOL date, but got none")
	}
	if status != "" {
		t.Errorf("Expected no status for an unparseable EOL date, but got '%s'", status)
	}
}

func TestEngineVersionMetricIncludesAWSAccountId(t *testing.T) {
	resetRDSEOLCache()
	ctx := context.TODO()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)
	mockClient.EXPECT().DescribeDBEngineVersion(ctx, "SQL", "1000").Return(nil, fmt.Errorf("engine not found"))

	x := RDSExporter{
		svcs:         []awsclient.Client{mockClient},
		configs:      []aws.Config{{Region: "foo"}},
		cache:        *NewMetricsCache(10 * time.Second),
		logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		awsAccountId: "1234567890",
	}

	x.addAllInstanceMetrics(ctx, 0, createTestDBInstances())

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
	mockClient.EXPECT().DescribePendingMaintenanceActionsAll(ctx).Return([]rds_types.ResourcePendingMaintenanceActions{
		{
			PendingMaintenanceActionDetails: []rds_types.PendingMaintenanceAction{{
				Action:      aws.String("something going on"),
				Description: aws.String("plumbing"),
			}},
			ResourceIdentifier: aws.String("::::::footest"),
		},
	}, nil)

	x := RDSExporter{
		svcs:    []awsclient.Client{mockClient},
		configs: []aws.Config{{Region: "foo"}},
		cache:   *NewMetricsCache(10 * time.Second),
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
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
	mockClient.EXPECT().DescribePendingMaintenanceActionsAll(ctx).Return([]rds_types.ResourcePendingMaintenanceActions{}, nil)

	x := RDSExporter{
		svcs:    []awsclient.Client{mockClient},
		configs: []aws.Config{{Region: "foo"}},
		cache:   *NewMetricsCache(10 * time.Second),
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	x.addAllPendingMaintenancesMetrics(ctx, 0, createTestDBInstances())
	metrics := x.cache.GetAllMetrics()
	assert.Len(t, metrics, 1)

	var dto dto.Metric
	metrics[0].Write(&dto)

	// Expecting no maintenance, thus 0 value
	assert.Equal(t, float64(0), *dto.Gauge.Value)
}
