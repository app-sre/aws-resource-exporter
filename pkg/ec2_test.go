package pkg

import (
	"context"
	"testing"

	"github.com/app-sre/aws-resource-exporter/pkg/awsclient/mock"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/servicequotas"
	servicequotasTypes "github.com/aws/aws-sdk-go-v2/service/servicequotas/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestGetAllTransitGateways(t *testing.T) {
	ctx := context.TODO()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)

	mockClient.EXPECT().DescribeTransitGateways(ctx, gomock.Any(), gomock.Any()).
		Return(&ec2.DescribeTransitGatewaysOutput{
			TransitGateways: []types.TransitGateway{{}},
		}, nil)

	gateways, err := getAllTransitGateways(mockClient, ctx)
	assert.Nil(t, err)
	assert.Len(t, gateways, 1)
}

func TestGetQuotaValue(t *testing.T) {
	ctx := context.TODO()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)

	mockClient.EXPECT().GetServiceQuota(ctx, gomock.Any(), gomock.Any()).Return(
		&servicequotas.GetServiceQuotaOutput{Quota: &servicequotasTypes.ServiceQuota{Value: aws.Float64(123.0)}}, nil,
	)

	quotaValue, err := getQuotaValue(mockClient, ec2ServiceCode, transitGatewayPerAccountQuotaCode, ctx)
	assert.Nil(t, err)
	assert.Equal(t, quotaValue, 123.0)
}

func TestGetQuotaValueError(t *testing.T) {
	ctx := context.TODO()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)

	mockClient.EXPECT().GetServiceQuota(ctx, gomock.Any(), gomock.Any()).Return(
		&servicequotas.GetServiceQuotaOutput{Quota: &servicequotasTypes.ServiceQuota{Value: nil}}, nil,
	)

	quotaValue, err := getQuotaValue(mockClient, ec2ServiceCode, transitGatewayPerAccountQuotaCode, ctx)
	assert.NotNil(t, err)
	assert.Equal(t, quotaValue, 0.0)
}
