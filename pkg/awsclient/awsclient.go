// inspired by https://github.com/openshift/aws-account-operator/blob/master/pkg/awsclient/client.go

package awsclient

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2_types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	elasticache_types "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/kafka"
	kafka_types "github.com/aws/aws-sdk-go-v2/service/kafka/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rds_types "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	route53_types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/aws/aws-sdk-go-v2/service/servicequotas"
)

//go:generate mockgen -source=./awsclient.go -destination=./mock/zz_generated.mock_client.go -package=mock

// Client is a wrapper object for actual AWS SDK clients to allow for easier testing.
type Client interface {
	// EC2
	GetTransitGatewaysCount(ctx context.Context, input *ec2.DescribeTransitGatewaysInput) (int, error)

	// VPC
	DescribeVpcsAll(ctx context.Context) ([]ec2_types.Vpc, error)
	DescribeVpcsCount(ctx context.Context) (int, error)
	DescribeSubnetsForVpc(ctx context.Context, vpcId string) ([]ec2_types.Subnet, error)
	DescribeSubnetsCountForVpc(ctx context.Context, vpcId string) (int, error)
	DescribeVpcEndpointsCountForVpc(ctx context.Context, vpcId string) (int, error)
	DescribeRouteTablesAll(ctx context.Context) ([]ec2_types.RouteTable, error)
	DescribeRouteTablesCountForVpc(ctx context.Context, vpcId string) (int, error)
	DescribeRouteTable(ctx context.Context, routeTableId string) (*ec2_types.RouteTable, error)
	DescribeVpc(ctx context.Context, vpcId string) (*ec2_types.Vpc, error)

	// RDS
	DescribeDBLogFilesAll(ctx context.Context, instanceId string) ([]*rds.DescribeDBLogFilesOutput, error)
	DescribePendingMaintenanceActionsAll(ctx context.Context) ([]rds_types.ResourcePendingMaintenanceActions, error)
	DescribeDBInstancesAll(ctx context.Context) ([]rds_types.DBInstance, error)

	// Service Quota
	GetServiceQuota(ctx context.Context, input *servicequotas.GetServiceQuotaInput, optFns ...func(*servicequotas.Options)) (*servicequotas.GetServiceQuotaOutput, error)

	//route53
	ListHostedZonesAll(ctx context.Context) ([]route53_types.HostedZone, error)
	GetHostedZoneLimit(ctx context.Context, input *route53.GetHostedZoneLimitInput, optFns ...func(*route53.Options)) (*route53.GetHostedZoneLimitOutput, error)

	// ElastiCache
	DescribeCacheClustersAll(ctx context.Context) ([]elasticache_types.CacheCluster, error)

	// MSK
	ListClustersAll(ctx context.Context) ([]kafka_types.ClusterInfo, error)

	// IAM
	GetAccountSummary(ctx context.Context, input *iam.GetAccountSummaryInput, optFns ...func(*iam.Options)) (*iam.GetAccountSummaryOutput, error)
}

type awsClient struct {
	ec2Client           *ec2.Client
	rdsClient           *rds.Client
	serviceQuotasClient *servicequotas.Client
	route53Client       *route53.Client
	elasticacheClient   *elasticache.Client
	mskClient           *kafka.Client
	iamClient           *iam.Client
	cfg                 aws.Config
}

func (c *awsClient) GetTransitGatewaysCount(ctx context.Context, input *ec2.DescribeTransitGatewaysInput) (int, error) {
	count := 0
	paginator := ec2.NewDescribeTransitGatewaysPaginator(c.ec2Client, input)
	for paginator.HasMorePages() {
		AwsExporterMetrics.IncrementRequests()
		result, err := paginator.NextPage(ctx)
		if err != nil {
			return count, err
		}
		count += len(result.TransitGateways)
	}
	return count, nil
}

// VPC Functions

func (c *awsClient) DescribeVpcsAll(ctx context.Context) ([]ec2_types.Vpc, error) {
	var vpcs []ec2_types.Vpc
	paginator := ec2.NewDescribeVpcsPaginator(c.ec2Client, &ec2.DescribeVpcsInput{})
	for paginator.HasMorePages() {
		AwsExporterMetrics.IncrementRequests()
		result, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		vpcs = append(vpcs, result.Vpcs...)
	}
	return vpcs, nil
}

func (c *awsClient) DescribeVpcsCount(ctx context.Context) (int, error) {
	count := 0
	paginator := ec2.NewDescribeVpcsPaginator(c.ec2Client, &ec2.DescribeVpcsInput{})
	for paginator.HasMorePages() {
		AwsExporterMetrics.IncrementRequests()
		result, err := paginator.NextPage(ctx)
		if err != nil {
			return 0, err
		}
		count += len(result.Vpcs)
	}
	return count, nil
}

func (c *awsClient) DescribeSubnetsForVpc(ctx context.Context, vpcId string) ([]ec2_types.Subnet, error) {
	var subnets []ec2_types.Subnet
	input := &ec2.DescribeSubnetsInput{
		Filters: []ec2_types.Filter{{
			Name:   aws.String("vpc-id"),
			Values: []string{vpcId},
		}},
	}
	paginator := ec2.NewDescribeSubnetsPaginator(c.ec2Client, input)
	for paginator.HasMorePages() {
		AwsExporterMetrics.IncrementRequests()
		result, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		subnets = append(subnets, result.Subnets...)
	}
	return subnets, nil
}

func (c *awsClient) DescribeSubnetsCountForVpc(ctx context.Context, vpcId string) (int, error) {
	count := 0
	input := &ec2.DescribeSubnetsInput{
		Filters: []ec2_types.Filter{{
			Name:   aws.String("vpc-id"),
			Values: []string{vpcId},
		}},
	}
	paginator := ec2.NewDescribeSubnetsPaginator(c.ec2Client, input)
	for paginator.HasMorePages() {
		AwsExporterMetrics.IncrementRequests()
		result, err := paginator.NextPage(ctx)
		if err != nil {
			return 0, err
		}
		count += len(result.Subnets)
	}
	return count, nil
}

func (c *awsClient) DescribeVpcEndpointsCountForVpc(ctx context.Context, vpcId string) (int, error) {
	count := 0
	input := &ec2.DescribeVpcEndpointsInput{
		Filters: []ec2_types.Filter{{
			Name:   aws.String("vpc-id"),
			Values: []string{vpcId},
		}},
	}
	paginator := ec2.NewDescribeVpcEndpointsPaginator(c.ec2Client, input)
	for paginator.HasMorePages() {
		AwsExporterMetrics.IncrementRequests()
		result, err := paginator.NextPage(ctx)
		if err != nil {
			return 0, err
		}
		count += len(result.VpcEndpoints)
	}
	return count, nil
}

func (c *awsClient) DescribeRouteTablesAll(ctx context.Context) ([]ec2_types.RouteTable, error) {
	var routeTables []ec2_types.RouteTable
	paginator := ec2.NewDescribeRouteTablesPaginator(c.ec2Client, &ec2.DescribeRouteTablesInput{})
	for paginator.HasMorePages() {
		AwsExporterMetrics.IncrementRequests()
		result, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		routeTables = append(routeTables, result.RouteTables...)
	}
	return routeTables, nil
}

func (c *awsClient) DescribeRouteTablesCountForVpc(ctx context.Context, vpcId string) (int, error) {
	count := 0
	input := &ec2.DescribeRouteTablesInput{
		Filters: []ec2_types.Filter{{
			Name:   aws.String("vpc-id"),
			Values: []string{vpcId},
		}},
	}
	paginator := ec2.NewDescribeRouteTablesPaginator(c.ec2Client, input)
	for paginator.HasMorePages() {
		AwsExporterMetrics.IncrementRequests()
		result, err := paginator.NextPage(ctx)
		if err != nil {
			return 0, err
		}
		count += len(result.RouteTables)
	}
	return count, nil
}

func (c *awsClient) DescribeRouteTable(ctx context.Context, routeTableId string) (*ec2_types.RouteTable, error) {
	AwsExporterMetrics.IncrementRequests()
	result, err := c.ec2Client.DescribeRouteTables(ctx, &ec2.DescribeRouteTablesInput{
		RouteTableIds: []string{routeTableId},
	})
	if err != nil {
		return nil, err
	}
	if len(result.RouteTables) != 1 {
		return nil, nil
	}
	return &result.RouteTables[0], nil
}

func (c *awsClient) DescribeVpc(ctx context.Context, vpcId string) (*ec2_types.Vpc, error) {
	AwsExporterMetrics.IncrementRequests()
	result, err := c.ec2Client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
		VpcIds: []string{vpcId},
	})
	if err != nil {
		return nil, err
	}
	if len(result.Vpcs) != 1 {
		return nil, nil
	}
	return &result.Vpcs[0], nil
}

func (c *awsClient) GetServiceQuota(ctx context.Context, input *servicequotas.GetServiceQuotaInput, optFns ...func(*servicequotas.Options)) (*servicequotas.GetServiceQuotaOutput, error) {
	return c.serviceQuotasClient.GetServiceQuota(ctx, input, optFns...)
}

func (c *awsClient) DescribeDBLogFilesAll(ctx context.Context, instanceId string) ([]*rds.DescribeDBLogFilesOutput, error) {
	input := &rds.DescribeDBLogFilesInput{
		DBInstanceIdentifier: &instanceId,
	}

	var logOutputs []*rds.DescribeDBLogFilesOutput
	paginator := rds.NewDescribeDBLogFilesPaginator(c.rdsClient, input)

	for paginator.HasMorePages() {
		AwsExporterMetrics.IncrementRequests()
		result, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		logOutputs = append(logOutputs, result)
	}

	return logOutputs, nil
}

func (c *awsClient) DescribePendingMaintenanceActionsAll(ctx context.Context) ([]rds_types.ResourcePendingMaintenanceActions, error) {
	input := &rds.DescribePendingMaintenanceActionsInput{}

	var instancesPendMaintActionsData []rds_types.ResourcePendingMaintenanceActions
	paginator := rds.NewDescribePendingMaintenanceActionsPaginator(c.rdsClient, input)

	for paginator.HasMorePages() {
		AwsExporterMetrics.IncrementRequests()
		result, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		instancesPendMaintActionsData = append(instancesPendMaintActionsData, result.PendingMaintenanceActions...)
	}

	return instancesPendMaintActionsData, nil
}

func (c *awsClient) DescribeDBInstancesAll(ctx context.Context) ([]rds_types.DBInstance, error) {
	input := &rds.DescribeDBInstancesInput{}

	var instances []rds_types.DBInstance
	paginator := rds.NewDescribeDBInstancesPaginator(c.rdsClient, input)

	for paginator.HasMorePages() {
		AwsExporterMetrics.IncrementRequests()
		result, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		instances = append(instances, result.DBInstances...)
	}

	return instances, nil
}

func (c *awsClient) ListHostedZonesAll(ctx context.Context) ([]route53_types.HostedZone, error) {
	input := &route53.ListHostedZonesInput{}

	var hostedZones []route53_types.HostedZone
	paginator := route53.NewListHostedZonesPaginator(c.route53Client, input)

	for paginator.HasMorePages() {
		AwsExporterMetrics.IncrementRequests()
		result, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		hostedZones = append(hostedZones, result.HostedZones...)
	}

	return hostedZones, nil
}

func (c *awsClient) GetHostedZoneLimit(ctx context.Context, input *route53.GetHostedZoneLimitInput, optFns ...func(*route53.Options)) (*route53.GetHostedZoneLimitOutput, error) {
	return c.route53Client.GetHostedZoneLimit(ctx, input, optFns...)
}

func (c *awsClient) DescribeCacheClustersAll(ctx context.Context) ([]elasticache_types.CacheCluster, error) {
	input := &elasticache.DescribeCacheClustersInput{}

	var clusters []elasticache_types.CacheCluster
	paginator := elasticache.NewDescribeCacheClustersPaginator(c.elasticacheClient, input)

	for paginator.HasMorePages() {
		AwsExporterMetrics.IncrementRequests()
		result, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		clusters = append(clusters, result.CacheClusters...)
	}

	return clusters, nil
}

func (c *awsClient) ListClustersAll(ctx context.Context) ([]kafka_types.ClusterInfo, error) {
	input := &kafka.ListClustersInput{}

	var clusters []kafka_types.ClusterInfo
	paginator := kafka.NewListClustersPaginator(c.mskClient, input)

	for paginator.HasMorePages() {
		AwsExporterMetrics.IncrementRequests()
		result, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		clusters = append(clusters, result.ClusterInfoList...)
	}

	return clusters, nil
}

func (c *awsClient) GetAccountSummary(ctx context.Context, input *iam.GetAccountSummaryInput, optFns ...func(*iam.Options)) (*iam.GetAccountSummaryOutput, error) {
	return c.iamClient.GetAccountSummary(ctx, input, optFns...)
}

func NewClientFromConfig(cfg aws.Config) Client {
	return &awsClient{
		ec2Client:           ec2.NewFromConfig(cfg),
		serviceQuotasClient: servicequotas.NewFromConfig(cfg),
		rdsClient:           rds.NewFromConfig(cfg),
		route53Client:       route53.NewFromConfig(cfg),
		elasticacheClient:   elasticache.NewFromConfig(cfg),
		mskClient:           kafka.NewFromConfig(cfg),
		iamClient:           iam.NewFromConfig(cfg),
		cfg:                 cfg,
	}
}

// Backwards compatibility function
func NewClient(ctx context.Context) (Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	return NewClientFromConfig(cfg), nil
}
