package pkg

import (
	"errors"
	"testing"
	"time"

	"github.com/app-sre/aws-resource-exporter/pkg/awsclient/mock"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/servicequotas"
	"github.com/go-kit/log"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestGetIAMMetrics_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIAM := mock.NewMockIAMClient(ctrl)
	mockServiceQuotas := mock.NewMockServiceQuotasClient(ctrl)

	mockIAM.EXPECT().ListRolesPagesWithContext(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx aws.Context, input *iam.ListRolesInput, fn func(*iam.ListRolesOutput, bool) bool, opts ...interface{}) error {
			fn(&iam.ListRolesOutput{Roles: []*iam.Role{{}, {}}}, true)
			return nil
		},
	)

	mockServiceQuotas.EXPECT().GetServiceQuotaWithContext(gomock.Any(), gomock.Any()).Return(
		&servicequotas.GetServiceQuotaOutput{
			Quota: &servicequotas.ServiceQuota{
				Value: aws.Float64(10),
			},
		}, nil,
	)

	iamExporter := &IAMExporter{
		iamClient:    mockIAM,
		sqClient:     mockServiceQuotas,
		logger:       log.NewNopLogger(),
		timeout:      10 * time.Second,
		interval:     15 * time.Second,
		awsAccountId: "123456789012",
	}

	roleCount, roleQuota, usagePercent, err := iamExporter.getIAMMetrics()

	assert.NoError(t, err)
	assert.Equal(t, 2, roleCount)
	assert.Equal(t, 10.0, roleQuota)
	assert.Equal(t, 20.0, usagePercent)
}

func TestGetIAMMetrics_ErrorHandling(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIAM := mock.NewMockIAMClient(ctrl)
	mockServiceQuotas := mock.NewMockServiceQuotasClient(ctrl)

	mockIAM.EXPECT().
		ListRolesPagesWithContext(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.New("IAM API error"))

	mockServiceQuotas.EXPECT().
		GetServiceQuotaWithContext(gomock.Any(), gomock.Any()).
		Return(
			&servicequotas.GetServiceQuotaOutput{
				Quota: &servicequotas.ServiceQuota{
					Value: aws.Float64(10),
				},
			}, nil,
		)

	iamExporter := &IAMExporter{
		iamClient:    mockIAM,
		sqClient:     mockServiceQuotas,
		logger:       log.NewNopLogger(),
		timeout:      10 * time.Second,
		interval:     15 * time.Second,
		awsAccountId: "123456789012",
	}

	_, _, _, err := iamExporter.getIAMMetrics()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "IAM API error")
}

func TestGetIAMMetrics_QuotaError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIAM := mock.NewMockIAMClient(ctrl)
	mockServiceQuotas := mock.NewMockServiceQuotasClient(ctrl)

	mockIAM.EXPECT().ListRolesPagesWithContext(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx aws.Context, input *iam.ListRolesInput, fn func(*iam.ListRolesOutput, bool) bool, opts ...interface{}) error {
			fn(&iam.ListRolesOutput{Roles: []*iam.Role{{}}}, true)
			return nil
		},
	)

	mockServiceQuotas.EXPECT().
		GetServiceQuotaWithContext(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("Quota API error"))

	iamExporter := &IAMExporter{
		iamClient:    mockIAM,
		sqClient:     mockServiceQuotas,
		logger:       log.NewNopLogger(),
		timeout:      10 * time.Second,
		interval:     15 * time.Second,
		awsAccountId: "123456789012",
	}

	roleCount, roleQuota, usagePercent, err := iamExporter.getIAMMetrics()

	assert.NoError(t, err)
	assert.Equal(t, 1, roleCount)
	assert.Equal(t, 0.0, roleQuota)
	assert.Equal(t, 0.0, usagePercent)
}
