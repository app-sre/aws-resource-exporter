package pkg

import (
	"context"
	"log/slog"
	"testing"

	"github.com/app-sre/aws-resource-exporter/pkg/awsclient/mock"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2_types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/servicequotas"
	servicequotas_types "github.com/aws/aws-sdk-go-v2/service/servicequotas/types"
	"github.com/golang/mock/gomock"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestGetTransitGatewaysCountWithContext(t *testing.T) {
	ctx := context.TODO()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)

	mockClient.EXPECT().GetTransitGatewaysCount(ctx, &ec2.DescribeTransitGatewaysInput{
		DryRun:     aws.Bool(false),
		MaxResults: aws.Int32(1000),
	}).Return(1, nil)

	count, err := getTransitGatewaysCountWithContext(mockClient, ctx)
	assert.Nil(t, err)
	assert.Equal(t, 1, count)
}

func TestGetQuotaValueWithContext(t *testing.T) {
	ctx := context.TODO()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)

	mockClient.EXPECT().GetServiceQuota(ctx,
		createGetServiceQuotaInput(ec2ServiceCode, transitGatewayPerAccountQuotaCode)).Return(
		&servicequotas.GetServiceQuotaOutput{Quota: &servicequotas_types.ServiceQuota{Value: aws.Float64(123.0)}}, nil,
	)

	quotaValue, err := getQuotaValueWithContext(mockClient, ec2ServiceCode, transitGatewayPerAccountQuotaCode, ctx)
	assert.Nil(t, err)
	assert.Equal(t, quotaValue, 123.0)
}

func TestGetQuotaValueWithContextError(t *testing.T) {
	ctx := context.TODO()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)

	mockClient.EXPECT().GetServiceQuota(ctx,
		createGetServiceQuotaInput(ec2ServiceCode, transitGatewayPerAccountQuotaCode)).Return(
		&servicequotas.GetServiceQuotaOutput{Quota: &servicequotas_types.ServiceQuota{Value: nil}}, nil,
	)

	quotaValue, err := getQuotaValueWithContext(mockClient, ec2ServiceCode, transitGatewayPerAccountQuotaCode, ctx)
	assert.NotNil(t, err)
	assert.Equal(t, quotaValue, 0.0)
}

func TestCollectInstanceBandwidth(t *testing.T) {
	ctx := context.TODO()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)

	instances := []ec2_types.Instance{
		{
			InstanceId:   aws.String("i-1234567890abcdef0"),
			InstanceType: ec2_types.InstanceTypeM5Xlarge,
		},
		{
			InstanceId:   aws.String("i-0987654321fedcba0"),
			InstanceType: ec2_types.InstanceTypeM5Xlarge,
		},
		{
			InstanceId:   aws.String("i-aabbccdd11223344"),
			InstanceType: ec2_types.InstanceTypeC5Large,
		},
	}

	mockClient.EXPECT().DescribeInstancesAll(ctx).Return(instances, nil)

	typeInfos := []ec2_types.InstanceTypeInfo{
		{
			InstanceType: ec2_types.InstanceTypeM5Xlarge,
			NetworkInfo: &ec2_types.NetworkInfo{
				NetworkCards: []ec2_types.NetworkCardInfo{
					{
						BaselineBandwidthInGbps: aws.Float64(10.0),
						PeakBandwidthInGbps:     aws.Float64(10.0),
					},
				},
			},
		},
		{
			InstanceType: ec2_types.InstanceTypeC5Large,
			NetworkInfo: &ec2_types.NetworkInfo{
				NetworkCards: []ec2_types.NetworkCardInfo{
					{
						BaselineBandwidthInGbps: aws.Float64(0.75),
						PeakBandwidthInGbps:     aws.Float64(10.0),
					},
				},
			},
		},
	}

	mockClient.EXPECT().DescribeInstanceTypes(ctx, gomock.Any()).Return(typeInfos, nil)

	// Initialize the metric descriptor
	EC2BandwidthLimitGbps = newTestBandwidthDesc()

	exporter := &EC2Exporter{
		cache: *NewMetricsCache(35 * 1000000000), // 35s
	}

	exporter.collectInstanceBandwidth(mockClient, "us-east-1", testLogger(), ctx)

	metrics := exporter.cache.GetAllMetrics()
	// Expect 3 metrics: 2 m5.xlarge (10 Gbps each) + 1 c5.large (0.75 Gbps)
	assert.Equal(t, 3, len(metrics))
}

func TestCollectInstanceBandwidthNoInstances(t *testing.T) {
	ctx := context.TODO()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)

	mockClient.EXPECT().DescribeInstancesAll(ctx).Return([]ec2_types.Instance{}, nil)

	exporter := &EC2Exporter{
		cache: *NewMetricsCache(35 * 1000000000),
	}

	// Should not call DescribeInstanceTypes when there are no instances
	exporter.collectInstanceBandwidth(mockClient, "us-east-1", testLogger(), ctx)

	metrics := exporter.cache.GetAllMetrics()
	assert.Equal(t, 0, len(metrics))
}

func newTestBandwidthDesc() *prometheus.Desc {
	return prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "ec2_instance_bandwidth_limit_gbps"),
		"Network bandwidth limit in Gbps for an EC2 instance",
		[]string{"aws_region", "instance_id", "instance_type"},
		map[string]string{"aws_account_id": "123456789012"},
	)
}

func testLogger() *slog.Logger {
	return slog.Default()
}
