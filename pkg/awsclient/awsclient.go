// inspired by https://github.com/openshift/aws-account-operator/blob/master/pkg/awsclient/client.go

package awsclient

import (
	"context"

	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/aws/aws-sdk-go/service/route53/route53iface"
	"github.com/aws/aws-sdk-go/service/servicequotas"
	"github.com/aws/aws-sdk-go/service/servicequotas/servicequotasiface"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
)

//go:generate mockgen -source=./awsclient.go -destination=./mock/zz_generated.mock_client.go -package=mock

// Client is a wrapper object for actual AWS SDK clients to allow for easier testing.
type Client interface {
	//EC2
	DescribeTransitGatewaysWithContext(ctx aws.Context, input *ec2.DescribeTransitGatewaysInput, opts ...request.Option) (*ec2.DescribeTransitGatewaysOutput, error)

	//RDS
	DescribeDBInstancesPagesWithContext(ctx aws.Context, input *rds.DescribeDBInstancesInput, fn func(*rds.DescribeDBInstancesOutput, bool) bool, opts ...request.Option) error
	DescribeDBLogFilesPagesWithContext(ctx aws.Context, input *rds.DescribeDBLogFilesInput, fn func(*rds.DescribeDBLogFilesOutput, bool) bool, opts ...request.Option) error
	DescribePendingMaintenanceActionsPagesWithContext(ctx aws.Context, input *rds.DescribePendingMaintenanceActionsInput, fn func(*rds.DescribePendingMaintenanceActionsOutput, bool) bool, opts ...request.Option) error
	DescribeDBLogFilesAll(ctx context.Context, instanceId string) ([]*rds.DescribeDBLogFilesOutput, error)
	DescribePendingMaintenanceActionsAll(ctx context.Context) ([]*rds.ResourcePendingMaintenanceActions, error)
	DescribeDBInstancesAll(ctx context.Context) ([]*rds.DBInstance, error)

	// Service Quota
	GetServiceQuotaWithContext(ctx aws.Context, input *servicequotas.GetServiceQuotaInput, opts ...request.Option) (*servicequotas.GetServiceQuotaOutput, error)

	//route53
	ListHostedZonesWithContext(ctx context.Context, input *route53.ListHostedZonesInput, opts ...request.Option) (*route53.ListHostedZonesOutput, error)
	GetHostedZoneLimitWithContext(ctx context.Context, input *route53.GetHostedZoneLimitInput, opts ...request.Option) (*route53.GetHostedZoneLimitOutput, error)
}

type Sts interface {
	AssumeRole(input *sts.AssumeRoleInput) (*sts.AssumeRoleOutput, error)
}

type awsClient struct {
	ec2Client           ec2iface.EC2API
	rdsClient           rds.RDS
	serviceQuotasClient servicequotasiface.ServiceQuotasAPI
	route53Client       route53iface.Route53API
}

type awsSts struct {
	sts stsiface.STSAPI
}

func (c *awsClient) DescribeTransitGatewaysWithContext(ctx aws.Context, input *ec2.DescribeTransitGatewaysInput, opts ...request.Option) (*ec2.DescribeTransitGatewaysOutput, error) {
	return c.ec2Client.DescribeTransitGatewaysWithContext(ctx, input, opts...)
}

func (c *awsClient) DescribeDBLogFilesPagesWithContext(ctx aws.Context, input *rds.DescribeDBLogFilesInput, fn func(*rds.DescribeDBLogFilesOutput, bool) bool, opts ...request.Option) error {
	return c.rdsClient.DescribeDBLogFilesPagesWithContext(ctx, input, fn, opts...)
}

func (c *awsClient) DescribeDBInstancesPagesWithContext(ctx aws.Context, input *rds.DescribeDBInstancesInput, fn func(*rds.DescribeDBInstancesOutput, bool) bool, opts ...request.Option) error {
	return c.rdsClient.DescribeDBInstancesPagesWithContext(ctx, input, fn, opts...)
}

func (c *awsClient) DescribePendingMaintenanceActionsPagesWithContext(ctx aws.Context, input *rds.DescribePendingMaintenanceActionsInput, fn func(*rds.DescribePendingMaintenanceActionsOutput, bool) bool, opts ...request.Option) error {
	return c.rdsClient.DescribePendingMaintenanceActionsPagesWithContext(ctx, input, fn, opts...)
}

func (c *awsClient) GetServiceQuotaWithContext(ctx aws.Context, input *servicequotas.GetServiceQuotaInput, opts ...request.Option) (*servicequotas.GetServiceQuotaOutput, error) {
	return c.serviceQuotasClient.GetServiceQuotaWithContext(ctx, input, opts...)
}

func (c *awsClient) DescribeDBLogFilesAll(ctx context.Context, instanceId string) ([]*rds.DescribeDBLogFilesOutput, error) {
	input := &rds.DescribeDBLogFilesInput{
		DBInstanceIdentifier: &instanceId,
	}

	var logOutPuts []*rds.DescribeDBLogFilesOutput
	err := c.DescribeDBLogFilesPagesWithContext(ctx, input, func(ddlo *rds.DescribeDBLogFilesOutput, b bool) bool {
		AwsExporterMetrics.IncrementRequests()
		logOutPuts = append(logOutPuts, ddlo)
		return true
	})

	if err != nil {
		AwsExporterMetrics.IncrementErrors()
		return nil, err
	}

	return logOutPuts, nil
}

func (c *awsClient) DescribePendingMaintenanceActionsAll(ctx context.Context) ([]*rds.ResourcePendingMaintenanceActions, error) {
	describePendingMaintInput := &rds.DescribePendingMaintenanceActionsInput{}

	var instancesPendMaintActionsData []*rds.ResourcePendingMaintenanceActions
	err := c.DescribePendingMaintenanceActionsPagesWithContext(ctx, describePendingMaintInput, func(dpm *rds.DescribePendingMaintenanceActionsOutput, b bool) bool {
		AwsExporterMetrics.IncrementRequests()
		instancesPendMaintActionsData = append(instancesPendMaintActionsData, dpm.PendingMaintenanceActions...)
		return true
	})

	if err != nil {
		AwsExporterMetrics.IncrementErrors()
		return nil, err
	}

	return instancesPendMaintActionsData, nil
}

func (c *awsClient) DescribeDBInstancesAll(ctx context.Context) ([]*rds.DBInstance, error) {
	input := &rds.DescribeDBInstancesInput{}

	var instances []*rds.DBInstance
	err := c.DescribeDBInstancesPagesWithContext(ctx, input, func(ddo *rds.DescribeDBInstancesOutput, b bool) bool {
		AwsExporterMetrics.IncrementRequests()
		instances = append(instances, ddo.DBInstances...)
		return true
	})
	if err != nil {
		AwsExporterMetrics.IncrementErrors()
		return nil, err
	}
	return instances, nil
}

func (c *awsClient) ListHostedZonesWithContext(ctx context.Context, input *route53.ListHostedZonesInput, opts ...request.Option) (*route53.ListHostedZonesOutput, error) {
	return c.route53Client.ListHostedZonesWithContext(ctx, input, opts...)
}

func (c *awsClient) GetHostedZoneLimitWithContext(ctx context.Context, input *route53.GetHostedZoneLimitInput, opts ...request.Option) (*route53.GetHostedZoneLimitOutput, error) {
	return c.route53Client.GetHostedZoneLimitWithContext(ctx, input, opts...)
}

func (s *awsSts) AssumeRole(input *sts.AssumeRoleInput) (*sts.AssumeRoleOutput, error) {
	return s.sts.AssumeRole(input)
}

func NewClientFromSession(sess *session.Session) Client {
	return &awsClient{
		ec2Client:           ec2.New(sess),
		serviceQuotasClient: servicequotas.New(sess),
		rdsClient:           *rds.New(sess),
		route53Client:       route53.New(sess),
	}
}

func NewClientForSTS(sess *session.Session) Sts {
	return &awsSts{
		sts: sts.New(sess),
	}
}
