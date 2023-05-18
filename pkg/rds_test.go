package pkg

import (
	"context"
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
			MaxAllocatedStorage:  aws.Int64(1024),
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
	x.addAllInstanceMetrics(0, instances)
	assert.Len(t, x.cache.GetAllMetrics(), 0)

	x.addAllInstanceMetrics(0, createTestDBInstances())
	assert.Len(t, x.cache.GetAllMetrics(), 9)
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
