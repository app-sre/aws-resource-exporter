package pkg

import (
	"context"
	"testing"

	"github.com/app-sre/aws-resource-exporter/pkg/awsclient/mock"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestGetHostedZoneLimitWithContext(t *testing.T) {
	ctx := context.TODO()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := mock.NewMockClient(ctrl)
	mockClient.EXPECT().GetHostedZoneLimitWithContext(ctx,
		createGetHostedZoneLimitInput(route53ServiceCode, hostedZonesQuotaCode)).Return(

		&route53.GetHostedZoneLimitOutput{
			Count: aws.Int64(12),
			Limit: &route53.HostedZoneLimit{
				Type:  aws.String("route53"),
				Value: aws.Int64(10)}}, nil)

	value, err := getHostedZoneValueWithContext(mockClient, route53ServiceCode, hostedZonesQuotaCode, ctx)
	assert.Nil(t, err)
	assert.Equal(t, value, int64(10))
}
