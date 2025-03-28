package pkg

import (
	"context"
	"testing"

	"github.com/app-sre/aws-resource-exporter/pkg/awsclient/mock"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/go-kit/log"
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

func TestListHostedZonesWithContext(t *testing.T) {
	ctx := context.TODO()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := mock.NewMockClient(ctrl)
	var logger log.Logger
	maxItems := "10"
	input := route53.ListHostedZonesInput{
		MaxItems: aws.String("10"),
	}
	mockClient.EXPECT().ListHostedZonesWithContext(ctx, createListHostedZonesWithContext(maxItems)).
		Return(&route53.ListHostedZonesOutput{
			HostedZones: []*route53.HostedZone{&route53.HostedZone{}},
			MaxItems:    aws.String("10"),
		}, nil)
	hostedZonesOutput, err := ListHostedZonesWithBackoff(mockClient, ctx, &input, maxRetries, logger)
	assert.Nil(t, err)
	assert.Equal(t, "10", *hostedZonesOutput.MaxItems)
}

func TestGetHostedZoneLimitWithBackoff(t *testing.T) {
	ctx := context.TODO()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := mock.NewMockClient(ctrl)
	var logger log.Logger

	mockClient.EXPECT().GetHostedZoneLimitWithContext(ctx, createGetHostedZoneLimitWithContext(route53ServiceCode, route53.HostedZoneLimitTypeMaxRrsetsByZone)).Return(
		&route53.GetHostedZoneLimitOutput{
			Limit: &route53.HostedZoneLimit{
				Type:  aws.String("route53"),
				Value: aws.Int64(10),
			},
		}, nil)

	hostedZoneLimitInput := &route53.GetHostedZoneLimitInput{
		HostedZoneId: aws.String("route53"),
		Type:         aws.String(route53.HostedZoneLimitTypeMaxRrsetsByZone),
	}

	actualResult, actualErr := GetHostedZoneLimitWithBackoff(mockClient, ctx, hostedZoneLimitInput.HostedZoneId, maxRetries, logger)
	assert.Nil(t, actualErr)
	assert.Equal(t, "route53", *actualResult.Limit.Type)

}

func TestListHostedZonesWithBackoff(t *testing.T) {
	ctx := context.TODO()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := mock.NewMockClient(ctrl)
	var logger log.Logger
	maxItems := "10"

	input := route53.ListHostedZonesInput{
		MaxItems: aws.String("10"),
	}

	mockClient.EXPECT().ListHostedZonesWithContext(ctx, createListHostedZonesWithContext(maxItems)).Return(
		&route53.ListHostedZonesOutput{
			HostedZones: []*route53.HostedZone{&route53.HostedZone{}},
			MaxItems:    aws.String("10"),
		}, nil)

	actualResult, actualErr := ListHostedZonesWithBackoff(mockClient, ctx, &input, maxRetries, logger)
	assert.Nil(t, actualErr)
	assert.Equal(t, "10", *actualResult.MaxItems)
}
