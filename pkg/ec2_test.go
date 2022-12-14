package pkg

import (
	"context"
	"testing"

	"github.com/app-sre/aws-resource-exporter/pkg/awsclient/mock"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/servicequotas"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestGetAllTransitGatewaysWithContext(t *testing.T) {
	ctx := context.TODO()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)

	mockClient.EXPECT().DescribeTransitGatewaysWithContext(ctx, createDescribeTransitGatewayInput()).
		Return(&ec2.DescribeTransitGatewaysOutput{
			TransitGateways: []*ec2.TransitGateway{&ec2.TransitGateway{}},
		}, nil)

	gateways, err := getAllTransitGatewaysWithContext(mockClient, ctx)
	assert.Nil(t, err)
	assert.Len(t, gateways, 1)
}

func TestGetQuotaValueWithContext(t *testing.T) {
	ctx := context.TODO()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)

	mockClient.EXPECT().GetServiceQuotaWithContext(ctx,
		createGetServiceQuotaInput(ec2ServiceCode, transitGatewayPerAccountQuotaCode)).Return(
		&servicequotas.GetServiceQuotaOutput{Quota: &servicequotas.ServiceQuota{Value: aws.Float64(123.0)}}, nil,
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

	mockClient.EXPECT().GetServiceQuotaWithContext(ctx,
		createGetServiceQuotaInput(ec2ServiceCode, transitGatewayPerAccountQuotaCode)).Return(
		&servicequotas.GetServiceQuotaOutput{Quota: &servicequotas.ServiceQuota{Value: nil}}, nil,
	)

	quotaValue, err := getQuotaValueWithContext(mockClient, ec2ServiceCode, transitGatewayPerAccountQuotaCode, ctx)
	assert.NotNil(t, err)
	assert.Equal(t, quotaValue, 0.0)
}
