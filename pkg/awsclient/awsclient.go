// inspired by https://github.com/openshift/aws-account-operator/blob/master/pkg/awsclient/client.go

package awsclient

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	elasticacheTypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	"github.com/aws/aws-sdk-go-v2/service/kafka"
	kafkaTypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/servicequotas"
)

//go:generate mockgen -source=./awsclient.go -destination=./mock/zz_generated.mock_client.go -package=mock

// Client is a wrapper object for actual AWS SDK clients to allow for easier testing.
type Client interface {
	//EC2
	DescribeTransitGateways(ctx context.Context, input *ec2.DescribeTransitGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeTransitGatewaysOutput, error)

	//RDS
	DescribeDBLogFiles(ctx context.Context, input *rds.DescribeDBLogFilesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBLogFilesOutput, error)
	DescribePendingMaintenanceActions(ctx context.Context, input *rds.DescribePendingMaintenanceActionsInput, optFns ...func(*rds.Options)) (*rds.DescribePendingMaintenanceActionsOutput, error)
	DescribeDBInstances(ctx context.Context, input *rds.DescribeDBInstancesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error)
	DescribeDBLogFilesAll(ctx context.Context, instanceId string) ([]*rds.DescribeDBLogFilesOutput, error)
	DescribePendingMaintenanceActionsAll(ctx context.Context) ([]types.ResourcePendingMaintenanceActions, error)
	DescribeDBInstancesAll(ctx context.Context) ([]types.DBInstance, error)

	// Service Quota
	GetServiceQuota(ctx context.Context, input *servicequotas.GetServiceQuotaInput, optFns ...func(*servicequotas.Options)) (*servicequotas.GetServiceQuotaOutput, error)

	//route53
	ListHostedZones(ctx context.Context, input *route53.ListHostedZonesInput, optFns ...func(*route53.Options)) (*route53.ListHostedZonesOutput, error)
	GetHostedZoneLimit(ctx context.Context, input *route53.GetHostedZoneLimitInput, optFns ...func(*route53.Options)) (*route53.GetHostedZoneLimitOutput, error)

	// ElastiCache
	DescribeCacheClusters(ctx context.Context, input *elasticache.DescribeCacheClustersInput, optFns ...func(*elasticache.Options)) (*elasticache.DescribeCacheClustersOutput, error)
	DescribeCacheClustersAll(ctx context.Context) ([]elasticacheTypes.CacheCluster, error)

	// MSK
	ListClusters(ctx context.Context, input *kafka.ListClustersInput, optFns ...func(*kafka.Options)) (*kafka.ListClustersOutput, error)
	ListClustersAll(ctx context.Context) ([]kafkaTypes.ClusterInfo, error)
}

type awsClient struct {
	ec2Client           *ec2.Client
	rdsClient           *rds.Client
	serviceQuotasClient *servicequotas.Client
	route53Client       *route53.Client
	elasticacheClient   *elasticache.Client
	mskClient           *kafka.Client
}

func (c *awsClient) DescribeTransitGateways(ctx context.Context, input *ec2.DescribeTransitGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeTransitGatewaysOutput, error) {
	return c.ec2Client.DescribeTransitGateways(ctx, input, optFns...)
}

func (c *awsClient) DescribeDBLogFiles(ctx context.Context, input *rds.DescribeDBLogFilesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBLogFilesOutput, error) {
	return c.rdsClient.DescribeDBLogFiles(ctx, input, optFns...)
}

func (c *awsClient) DescribeDBInstances(ctx context.Context, input *rds.DescribeDBInstancesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
	return c.rdsClient.DescribeDBInstances(ctx, input, optFns...)
}

func (c *awsClient) DescribePendingMaintenanceActions(ctx context.Context, input *rds.DescribePendingMaintenanceActionsInput, optFns ...func(*rds.Options)) (*rds.DescribePendingMaintenanceActionsOutput, error) {
	return c.rdsClient.DescribePendingMaintenanceActions(ctx, input, optFns...)
}

func (c *awsClient) GetServiceQuota(ctx context.Context, input *servicequotas.GetServiceQuotaInput, optFns ...func(*servicequotas.Options)) (*servicequotas.GetServiceQuotaOutput, error) {
	return c.serviceQuotasClient.GetServiceQuota(ctx, input, optFns...)
}

func (c *awsClient) DescribeDBLogFilesAll(ctx context.Context, instanceId string) ([]*rds.DescribeDBLogFilesOutput, error) {
	paginator := rds.NewDescribeDBLogFilesPaginator(c.rdsClient, &rds.DescribeDBLogFilesInput{
		DBInstanceIdentifier: &instanceId,
	})

	var logOutPuts []*rds.DescribeDBLogFilesOutput
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			AwsExporterMetrics.IncrementErrors()
			return nil, err
		}
		logOutPuts = append(logOutPuts, output)
		AwsExporterMetrics.IncrementRequests()
	}

	return logOutPuts, nil
}

func (c *awsClient) DescribePendingMaintenanceActionsAll(ctx context.Context) ([]types.ResourcePendingMaintenanceActions, error) {
	paginator := rds.NewDescribePendingMaintenanceActionsPaginator(c.rdsClient, &rds.DescribePendingMaintenanceActionsInput{})

	var instancesPendMaintActionsData []types.ResourcePendingMaintenanceActions
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			AwsExporterMetrics.IncrementErrors()
			return nil, err
		}
		instancesPendMaintActionsData = append(instancesPendMaintActionsData, output.PendingMaintenanceActions...)
		AwsExporterMetrics.IncrementRequests()
	}

	return instancesPendMaintActionsData, nil
}

func (c *awsClient) DescribeDBInstancesAll(ctx context.Context) ([]types.DBInstance, error) {
	paginator := rds.NewDescribeDBInstancesPaginator(c.rdsClient, &rds.DescribeDBInstancesInput{})

	var instances []types.DBInstance
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			AwsExporterMetrics.IncrementErrors()
			return nil, err
		}
		instances = append(instances, output.DBInstances...)
		AwsExporterMetrics.IncrementRequests()
	}
	return instances, nil
}

func (c *awsClient) ListHostedZones(ctx context.Context, input *route53.ListHostedZonesInput, optFns ...func(*route53.Options)) (*route53.ListHostedZonesOutput, error) {
	return c.route53Client.ListHostedZones(ctx, input, optFns...)
}

func (c *awsClient) GetHostedZoneLimit(ctx context.Context, input *route53.GetHostedZoneLimitInput, optFns ...func(*route53.Options)) (*route53.GetHostedZoneLimitOutput, error) {
	return c.route53Client.GetHostedZoneLimit(ctx, input, optFns...)
}

func (c *awsClient) DescribeCacheClusters(ctx context.Context, input *elasticache.DescribeCacheClustersInput, optFns ...func(*elasticache.Options)) (*elasticache.DescribeCacheClustersOutput, error) {
	return c.elasticacheClient.DescribeCacheClusters(ctx, input, optFns...)
}

func (c *awsClient) DescribeCacheClustersAll(ctx context.Context) ([]elasticacheTypes.CacheCluster, error) {
	paginator := elasticache.NewDescribeCacheClustersPaginator(c.elasticacheClient, &elasticache.DescribeCacheClustersInput{})

	var clusters []elasticacheTypes.CacheCluster
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			AwsExporterMetrics.IncrementErrors()
			return nil, err
		}
		clusters = append(clusters, output.CacheClusters...)
		AwsExporterMetrics.IncrementRequests()
	}
	return clusters, nil
}

func (c *awsClient) ListClusters(ctx context.Context, input *kafka.ListClustersInput, optFns ...func(*kafka.Options)) (*kafka.ListClustersOutput, error) {
	return c.mskClient.ListClusters(ctx, input, optFns...)
}

func (c *awsClient) ListClustersAll(ctx context.Context) ([]kafkaTypes.ClusterInfo, error) {
	paginator := kafka.NewListClustersPaginator(c.mskClient, &kafka.ListClustersInput{})

	var clusters []kafkaTypes.ClusterInfo
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			AwsExporterMetrics.IncrementErrors()
			return nil, err
		}
		clusters = append(clusters, output.ClusterInfoList...)
		AwsExporterMetrics.IncrementRequests()
	}

	return clusters, nil
}

func NewClientFromConfig(cfg aws.Config) Client {
	return &awsClient{
		ec2Client:           ec2.NewFromConfig(cfg),
		serviceQuotasClient: servicequotas.NewFromConfig(cfg),
		rdsClient:           rds.NewFromConfig(cfg),
		route53Client:       route53.NewFromConfig(cfg),
		elasticacheClient:   elasticache.NewFromConfig(cfg),
		mskClient:           kafka.NewFromConfig(cfg),
	}
}

func (c *awsClient) DescribeCacheClustersPagesWithContext(ctx context.Context, input *elasticache.DescribeCacheClustersInput, fn func(*elasticache.DescribeCacheClustersOutput, bool) bool) error {
	paginator := elasticache.NewDescribeCacheClustersPaginator(c.elasticacheClient, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return err
		}
		if !fn(page, paginator.HasMorePages()) {
			break
		}
	}
	return nil
}

func (c *awsClient) ListClustersPagesWithContext(ctx context.Context, input *kafka.ListClustersInput, fn func(*kafka.ListClustersOutput, bool) bool) error {
	paginator := kafka.NewListClustersPaginator(c.mskClient, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return err
		}
		if !fn(page, paginator.HasMorePages()) {
			break
		}
	}
	return nil
}
