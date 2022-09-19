// inspired by https://github.com/openshift/aws-account-operator/blob/master/pkg/awsclient/client.go

package awsclient

import (
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/servicequotas"
	"github.com/aws/aws-sdk-go/service/servicequotas/servicequotasiface"

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

	// Service Quota
	GetServiceQuota(*servicequotas.GetServiceQuotaInput) (*servicequotas.GetServiceQuotaOutput, error)
	GetServiceQuotaWithContext(ctx aws.Context, input *servicequotas.GetServiceQuotaInput, opts ...request.Option) (*servicequotas.GetServiceQuotaOutput, error)
	RequestServiceQuotaIncrease(*servicequotas.RequestServiceQuotaIncreaseInput) (*servicequotas.RequestServiceQuotaIncreaseOutput, error)
	ListRequestedServiceQuotaChangeHistory(*servicequotas.ListRequestedServiceQuotaChangeHistoryInput) (*servicequotas.ListRequestedServiceQuotaChangeHistoryOutput, error)
	ListRequestedServiceQuotaChangeHistoryByQuota(*servicequotas.ListRequestedServiceQuotaChangeHistoryByQuotaInput) (*servicequotas.ListRequestedServiceQuotaChangeHistoryByQuotaOutput, error)
}

type awsClient struct {
	ec2Client           ec2iface.EC2API
	serviceQuotasClient servicequotasiface.ServiceQuotasAPI
}

// NewAwsClientInput input for new aws client
type NewAwsClientInput struct {
	AwsCredsSecretIDKey     string
	AwsCredsSecretAccessKey string
	AwsToken                string
	AwsRegion               string
	SecretName              string
	NameSpace               string
}

func (c *awsClient) DescribeTransitGatewaysWithContext(ctx aws.Context, input *ec2.DescribeTransitGatewaysInput, opts ...request.Option) (*ec2.DescribeTransitGatewaysOutput, error) {
	return c.ec2Client.DescribeTransitGatewaysWithContext(ctx, input, opts...)
}

func (c *awsClient) DeleteSubnet(input *ec2.DeleteSubnetInput) (*ec2.DeleteSubnetOutput, error) {
	return c.ec2Client.DeleteSubnet(input)
}

func (c *awsClient) GetServiceQuota(input *servicequotas.GetServiceQuotaInput) (*servicequotas.GetServiceQuotaOutput, error) {
	return c.serviceQuotasClient.GetServiceQuota(input)
}

func (c *awsClient) GetServiceQuotaWithContext(ctx aws.Context, input *servicequotas.GetServiceQuotaInput, opts ...request.Option) (*servicequotas.GetServiceQuotaOutput, error) {
	return c.serviceQuotasClient.GetServiceQuotaWithContext(ctx, input, opts...)
}

func (c *awsClient) RequestServiceQuotaIncrease(input *servicequotas.RequestServiceQuotaIncreaseInput) (*servicequotas.RequestServiceQuotaIncreaseOutput, error) {
	return c.serviceQuotasClient.RequestServiceQuotaIncrease(input)
}

func (c *awsClient) ListRequestedServiceQuotaChangeHistory(input *servicequotas.ListRequestedServiceQuotaChangeHistoryInput) (*servicequotas.ListRequestedServiceQuotaChangeHistoryOutput, error) {
	return c.serviceQuotasClient.ListRequestedServiceQuotaChangeHistory(input)
}

func (c *awsClient) ListRequestedServiceQuotaChangeHistoryByQuota(input *servicequotas.ListRequestedServiceQuotaChangeHistoryByQuotaInput) (*servicequotas.ListRequestedServiceQuotaChangeHistoryByQuotaOutput, error) {
	return c.serviceQuotasClient.ListRequestedServiceQuotaChangeHistoryByQuota(input)
}

func NewClientFromSession(sess *session.Session) Client {
	return &awsClient{
		ec2Client:           ec2.New(sess),
		serviceQuotasClient: servicequotas.New(sess),
	}
}
