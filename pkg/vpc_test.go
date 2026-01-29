package pkg

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/app-sre/aws-resource-exporter/pkg/awsclient"
	"github.com/app-sre/aws-resource-exporter/pkg/awsclient/mock"
	"github.com/aws/aws-sdk-go-v2/aws"
	ec2_types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/servicequotas"
	servicequotas_types "github.com/aws/aws-sdk-go-v2/service/servicequotas/types"
	"github.com/golang/mock/gomock"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
)

var vpcTestTimeout = 10 * time.Second

func createTestVPCExporter(svcs []awsclient.Client) *VPCExporter {
	configs := make([]aws.Config, len(svcs))
	for i := range svcs {
		configs[i] = aws.Config{Region: "us-east-1"}
	}

	return &VPCExporter{
		awsAccountId: "123456789012",
		configs:      configs,
		svcs:         svcs,
		cache:        *NewMetricsCache(vpcTestTimeout),
		timeout:      vpcTestTimeout,
		logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestCollectVpcsPerRegionUsage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)
	mockClient.EXPECT().DescribeVpcsCount(gomock.Any()).Return(5, nil)

	e := createTestVPCExporter([]awsclient.Client{mockClient})
	e.VpcsPerRegionUsage = createTestVpcDesc("vpc_vpcsperregion_usage", []string{"aws_region"})

	e.collectVpcsPerRegionUsage(mockClient, "us-east-1")

	metrics := e.cache.GetAllMetrics()
	assert.Len(t, metrics, 1)

	var m dto.Metric
	metrics[0].Write(&m)
	assert.Equal(t, float64(5), *m.Gauge.Value)
}

func TestCollectSubnetsPerVpcUsage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)
	mockClient.EXPECT().DescribeSubnetsCountForVpc(gomock.Any(), "vpc-123").Return(3, nil)

	e := createTestVPCExporter([]awsclient.Client{mockClient})
	e.SubnetsPerVpcUsage = createTestVpcDesc("vpc_subnetspervpc_usage", []string{"aws_region", "vpcid"})

	vpc := ec2_types.Vpc{VpcId: aws.String("vpc-123")}
	e.collectSubnetsPerVpcUsage(mockClient, vpc, "us-east-1")

	metrics := e.cache.GetAllMetrics()
	assert.Len(t, metrics, 1)

	var m dto.Metric
	metrics[0].Write(&m)
	assert.Equal(t, float64(3), *m.Gauge.Value)
}

func TestCollectInterfaceVpcEndpointsPerVpcUsage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)
	mockClient.EXPECT().DescribeVpcEndpointsCountForVpc(gomock.Any(), "vpc-123").Return(7, nil)

	e := createTestVPCExporter([]awsclient.Client{mockClient})
	e.InterfaceVpcEndpointsPerVpcUsage = createTestVpcDesc("vpc_interfacevpcendpointspervpc_usage", []string{"aws_region", "vpcid"})

	vpc := ec2_types.Vpc{VpcId: aws.String("vpc-123")}
	e.collectInterfaceVpcEndpointsPerVpcUsage(mockClient, vpc, "us-east-1")

	metrics := e.cache.GetAllMetrics()
	assert.Len(t, metrics, 1)

	var m dto.Metric
	metrics[0].Write(&m)
	assert.Equal(t, float64(7), *m.Gauge.Value)
}

func TestCollectRoutesTablesPerVpcUsage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)
	mockClient.EXPECT().DescribeRouteTablesCountForVpc(gomock.Any(), "vpc-123").Return(4, nil)

	e := createTestVPCExporter([]awsclient.Client{mockClient})
	e.RouteTablesPerVpcUsage = createTestVpcDesc("vpc_routetablespervpc_usage", []string{"aws_region", "vpcid"})

	vpc := ec2_types.Vpc{VpcId: aws.String("vpc-123")}
	e.collectRoutesTablesPerVpcUsage(mockClient, vpc, "us-east-1")

	metrics := e.cache.GetAllMetrics()
	assert.Len(t, metrics, 1)

	var m dto.Metric
	metrics[0].Write(&m)
	assert.Equal(t, float64(4), *m.Gauge.Value)
}

func TestCollectRoutesPerRouteTableUsage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)
	routeTable := &ec2_types.RouteTable{
		RouteTableId: aws.String("rtb-123"),
		VpcId:        aws.String("vpc-123"),
		Routes: []ec2_types.Route{
			{DestinationCidrBlock: aws.String("10.0.0.0/16")},
			{DestinationCidrBlock: aws.String("0.0.0.0/0")},
		},
	}
	mockClient.EXPECT().DescribeRouteTable(gomock.Any(), "rtb-123").Return(routeTable, nil)

	e := createTestVPCExporter([]awsclient.Client{mockClient})
	e.RoutesPerRouteTableUsage = createTestVpcDesc("vpc_routesperroutetable_usage", []string{"aws_region", "vpcid", "routetableid"})

	rtb := ec2_types.RouteTable{
		RouteTableId: aws.String("rtb-123"),
		VpcId:        aws.String("vpc-123"),
	}
	e.collectRoutesPerRouteTableUsage(mockClient, rtb, "us-east-1")

	metrics := e.cache.GetAllMetrics()
	assert.Len(t, metrics, 1)

	var m dto.Metric
	metrics[0].Write(&m)
	assert.Equal(t, float64(2), *m.Gauge.Value)
}

func TestCollectIPv4BlocksPerVpcUsage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)
	descVpc := &ec2_types.Vpc{
		VpcId: aws.String("vpc-123"),
		CidrBlockAssociationSet: []ec2_types.VpcCidrBlockAssociation{
			{CidrBlock: aws.String("10.0.0.0/16")},
			{CidrBlock: aws.String("10.1.0.0/16")},
		},
	}
	mockClient.EXPECT().DescribeVpc(gomock.Any(), "vpc-123").Return(descVpc, nil)

	e := createTestVPCExporter([]awsclient.Client{mockClient})
	e.IPv4BlocksPerVpcUsage = createTestVpcDesc("vpc_ipv4blockspervpc_usage", []string{"aws_region", "vpcid"})

	vpc := ec2_types.Vpc{VpcId: aws.String("vpc-123")}
	e.collectIPv4BlocksPerVpcUsage(mockClient, vpc, "us-east-1")

	metrics := e.cache.GetAllMetrics()
	assert.Len(t, metrics, 1)

	var m dto.Metric
	metrics[0].Write(&m)
	assert.Equal(t, float64(2), *m.Gauge.Value)
}

func TestCollectIPv4AddressesPerSubnetUsage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)
	subnets := []ec2_types.Subnet{
		{
			SubnetId:                aws.String("subnet-123"),
			CidrBlock:               aws.String("10.0.0.0/24"),
			AvailableIpAddressCount: aws.Int32(200),
		},
	}
	mockClient.EXPECT().DescribeSubnetsForVpc(gomock.Any(), "vpc-123").Return(subnets, nil)

	e := createTestVPCExporter([]awsclient.Client{mockClient})
	e.IPv4AddressesPerSubnetCapacity = createTestVpcDesc("vpc_ipv4addressespersubnet_capacity", []string{"aws_region", "vpcid", "subnetid"})
	e.IPv4AddressesPerSubnetUsage = createTestVpcDesc("vpc_ipv4addressespersubnet_usage", []string{"aws_region", "vpcid", "subnetid"})

	vpc := ec2_types.Vpc{VpcId: aws.String("vpc-123")}
	e.collectIPv4AddressesPerSubnetUsage(mockClient, vpc, "us-east-1")

	metrics := e.cache.GetAllMetrics()
	assert.Len(t, metrics, 2)

	// /24 = 256 IPs, minus 5 reserved = 251 usable
	// 200 available, so 51 used
	var capacityValue, usageValue float64
	for _, metric := range metrics {
		var m dto.Metric
		metric.Write(&m)
		v := m.GetGauge().GetValue()
		if v == 251 {
			capacityValue = v
		} else {
			usageValue = v
		}
	}
	assert.Equal(t, float64(251), capacityValue)
	assert.Equal(t, float64(51), usageValue)
}

func TestGetQuotaValue(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)
	mockClient.EXPECT().GetServiceQuota(gomock.Any(), &servicequotas.GetServiceQuotaInput{
		QuotaCode:   aws.String(QUOTA_VPCS_PER_REGION),
		ServiceCode: aws.String(SERVICE_CODE_VPC),
	}).Return(&servicequotas.GetServiceQuotaOutput{
		Quota: &servicequotas_types.ServiceQuota{Value: aws.Float64(100.0)},
	}, nil)

	e := createTestVPCExporter([]awsclient.Client{mockClient})

	quota, err := e.GetQuotaValue(mockClient, SERVICE_CODE_VPC, QUOTA_VPCS_PER_REGION)
	assert.Nil(t, err)
	assert.Equal(t, 100.0, quota)
}

func TestGetQuotaValueNilQuota(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)
	mockClient.EXPECT().GetServiceQuota(gomock.Any(), gomock.Any()).Return(&servicequotas.GetServiceQuotaOutput{
		Quota: &servicequotas_types.ServiceQuota{Value: nil},
	}, nil)

	e := createTestVPCExporter([]awsclient.Client{mockClient})

	quota, err := e.GetQuotaValue(mockClient, SERVICE_CODE_VPC, QUOTA_VPCS_PER_REGION)
	assert.NotNil(t, err)
	assert.Equal(t, 0.0, quota)
}

func TestCollectVpcsPerRegionQuota(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)
	mockClient.EXPECT().GetServiceQuota(gomock.Any(), &servicequotas.GetServiceQuotaInput{
		QuotaCode:   aws.String(QUOTA_VPCS_PER_REGION),
		ServiceCode: aws.String(SERVICE_CODE_VPC),
	}).Return(&servicequotas.GetServiceQuotaOutput{
		Quota: &servicequotas_types.ServiceQuota{Value: aws.Float64(5.0)},
	}, nil)

	e := createTestVPCExporter([]awsclient.Client{mockClient})
	e.VpcsPerRegionQuota = createTestVpcDesc("vpc_vpcsperregion_quota", []string{"aws_region"})

	e.collectVpcsPerRegionQuota(mockClient, "us-east-1")

	metrics := e.cache.GetAllMetrics()
	assert.Len(t, metrics, 1)

	var m dto.Metric
	metrics[0].Write(&m)
	assert.Equal(t, float64(5), *m.Gauge.Value)
}

func TestDescribeVpcsAll(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)
	vpcs := []ec2_types.Vpc{
		{VpcId: aws.String("vpc-1")},
		{VpcId: aws.String("vpc-2")},
		{VpcId: aws.String("vpc-3")},
	}
	mockClient.EXPECT().DescribeVpcsAll(gomock.Any()).Return(vpcs, nil)

	ctx := context.TODO()
	result, err := mockClient.DescribeVpcsAll(ctx)

	assert.Nil(t, err)
	assert.Len(t, result, 3)
	assert.Equal(t, "vpc-1", *result[0].VpcId)
	assert.Equal(t, "vpc-2", *result[1].VpcId)
	assert.Equal(t, "vpc-3", *result[2].VpcId)
}

func TestDescribeRouteTablesAll(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)
	routeTables := []ec2_types.RouteTable{
		{RouteTableId: aws.String("rtb-1"), VpcId: aws.String("vpc-1")},
		{RouteTableId: aws.String("rtb-2"), VpcId: aws.String("vpc-1")},
	}
	mockClient.EXPECT().DescribeRouteTablesAll(gomock.Any()).Return(routeTables, nil)

	ctx := context.TODO()
	result, err := mockClient.DescribeRouteTablesAll(ctx)

	assert.Nil(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "rtb-1", *result[0].RouteTableId)
	assert.Equal(t, "rtb-2", *result[1].RouteTableId)
}

func createTestVpcDesc(name string, labels []string) *prometheus.Desc {
	return prometheus.NewDesc(prometheus.BuildFQName(namespace, "", name), "test description", labels, nil)
}
