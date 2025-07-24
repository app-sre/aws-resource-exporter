package pkg

import (
	"context"
	"log/slog"
	"testing"

	"github.com/app-sre/aws-resource-exporter/pkg/awsclient/mock"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestGetHostedZoneLimit(t *testing.T) {
	ctx := context.TODO()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := mock.NewMockClient(ctrl)
	mockClient.EXPECT().GetHostedZoneLimit(ctx,
		&route53.GetHostedZoneLimitInput{
			HostedZoneId: aws.String(route53ServiceCode),
			Type:         types.HostedZoneLimitTypeMaxRrsetsByZone,
		}).Return(

		&route53.GetHostedZoneLimitOutput{
			Count: 12,
			Limit: &types.HostedZoneLimit{
				Type:  types.HostedZoneLimitTypeMaxRrsetsByZone,
				Value: aws.Int64(10)}}, nil)

	value, err := getHostedZoneValueWithContext(mockClient, route53ServiceCode, types.HostedZoneLimitTypeMaxRrsetsByZone, ctx)
	assert.Nil(t, err)
	assert.Equal(t, value, int64(10))
}

func TestListHostedZones(t *testing.T) {
	ctx := context.TODO()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := mock.NewMockClient(ctrl)
	var logger *slog.Logger
	input := route53.ListHostedZonesInput{
		MaxItems: aws.Int32(10),
	}
	mockClient.EXPECT().ListHostedZones(ctx, &input).
		Return(&route53.ListHostedZonesOutput{
			HostedZones: []types.HostedZone{{}},
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

	mockClient.EXPECT().GetHostedZoneLimit(ctx, &route53.GetHostedZoneLimitInput{
		HostedZoneId: aws.String(route53ServiceCode),
		Type:         types.HostedZoneLimitTypeMaxRrsetsByZone,
	}).Return(
		&route53.GetHostedZoneLimitOutput{
			Limit: &types.HostedZoneLimit{
				Type:  types.HostedZoneLimitTypeMaxRrsetsByZone,
				Value: aws.Int64(10),
			},
		}, nil)

	hostedZoneLimitInput := &route53.GetHostedZoneLimitInput{
		HostedZoneId: aws.String("route53"),
		Type:         types.HostedZoneLimitTypeMaxRrsetsByZone,
	}

	actualResult, actualErr := GetHostedZoneLimitWithBackoff(mockClient, ctx, hostedZoneLimitInput.HostedZoneId, maxRetries, logger)
	assert.Nil(t, actualErr)
	assert.Equal(t, types.HostedZoneLimitTypeMaxRrsetsByZone, actualResult.Limit.Type)
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

	mockClient.EXPECT().ListHostedZones(ctx, &input).Return(
		&route53.ListHostedZonesOutput{
			HostedZones: []types.HostedZone{{}},
			MaxItems:    aws.Int32(10),
		}, nil)

	actualResult, actualErr := ListHostedZonesWithBackoff(mockClient, ctx, &input, maxRetries, logger)
	assert.Nil(t, actualErr)
	assert.Equal(t, int32(10), *actualResult.MaxItems)
}
