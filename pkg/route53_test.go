package pkg

import (
	"context"
	"log/slog"
	"testing"

	"github.com/app-sre/aws-resource-exporter/pkg/awsclient/mock"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	route53_types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestGetHostedZoneLimitWithContext(t *testing.T) {
	ctx := context.TODO()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := mock.NewMockClient(ctrl)
	mockClient.EXPECT().GetHostedZoneLimit(ctx,
		createGetHostedZoneLimitInput("hostedZoneId", string(route53_types.HostedZoneLimitTypeMaxRrsetsByZone))).Return(

		&route53.GetHostedZoneLimitOutput{
			Count: 12,
			Limit: &route53_types.HostedZoneLimit{
				Type:  route53_types.HostedZoneLimitTypeMaxRrsetsByZone,
				Value: aws.Int64(10)}}, nil)

	value, err := getHostedZoneValueWithContext(mockClient, "hostedZoneId", string(route53_types.HostedZoneLimitTypeMaxRrsetsByZone), ctx)
	assert.Nil(t, err)
	assert.Equal(t, value, int64(10))
}

func TestListHostedZonesWithContext(t *testing.T) {
	ctx := context.TODO()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := mock.NewMockClient(ctrl)
	var logger *slog.Logger
	input := route53.ListHostedZonesInput{
		MaxItems: aws.Int32(10),
	}
	mockClient.EXPECT().ListHostedZones(ctx, createListHostedZones(10)).
		Return(&route53.ListHostedZonesOutput{
			HostedZones: []route53_types.HostedZone{{}},
			MaxItems:    aws.Int32(10),
		}, nil)
	hostedZonesOutput, err := ListHostedZonesWithBackoff(mockClient, ctx, &input, maxRetries, logger)
	assert.Nil(t, err)
	assert.Equal(t, int32(10), *hostedZonesOutput.MaxItems)
}

func TestGetHostedZoneLimitWithBackoff(t *testing.T) {
	ctx := context.TODO()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := mock.NewMockClient(ctrl)
	var logger *slog.Logger

	mockClient.EXPECT().GetHostedZoneLimit(ctx, createGetHostedZoneLimit("route53", string(route53_types.HostedZoneLimitTypeMaxRrsetsByZone))).Return(
		&route53.GetHostedZoneLimitOutput{
			Limit: &route53_types.HostedZoneLimit{
				Type:  route53_types.HostedZoneLimitTypeMaxRrsetsByZone,
				Value: aws.Int64(10),
			},
		}, nil)

	hostedZoneLimitInput := &route53.GetHostedZoneLimitInput{
		HostedZoneId: aws.String("route53"),
		Type:         route53_types.HostedZoneLimitTypeMaxRrsetsByZone,
	}

	actualResult, actualErr := GetHostedZoneLimitWithBackoff(mockClient, ctx, hostedZoneLimitInput.HostedZoneId, maxRetries, logger)
	assert.Nil(t, actualErr)
	assert.Equal(t, route53_types.HostedZoneLimitTypeMaxRrsetsByZone, actualResult.Limit.Type)

}

func TestListHostedZonesWithBackoff(t *testing.T) {
	ctx := context.TODO()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := mock.NewMockClient(ctrl)
	var logger *slog.Logger

	input := route53.ListHostedZonesInput{
		MaxItems: aws.Int32(10),
	}

	mockClient.EXPECT().ListHostedZones(ctx, createListHostedZones(10)).Return(
		&route53.ListHostedZonesOutput{
			HostedZones: []route53_types.HostedZone{{}},
			MaxItems:    aws.Int32(10),
		}, nil)

	actualResult, actualErr := ListHostedZonesWithBackoff(mockClient, ctx, &input, maxRetries, logger)
	assert.Nil(t, actualErr)
	assert.Equal(t, int32(10), *actualResult.MaxItems)
}
