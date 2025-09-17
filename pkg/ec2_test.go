package pkg

import (
	"context"
	"testing"

	"github.com/app-sre/aws-resource-exporter/pkg/awsclient/mock"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/servicequotas"
	servicequotas_types "github.com/aws/aws-sdk-go-v2/service/servicequotas/types"
	"github.com/golang/mock/gomock"
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
