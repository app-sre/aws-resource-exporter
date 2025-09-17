package pkg

import (
	"context"
	"testing"

	"github.com/app-sre/aws-resource-exporter/pkg/awsclient/mock"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestGetIAMAccountSummary_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)
	ctx := context.TODO()

	mockClient.EXPECT().
		GetAccountSummary(ctx, &iam.GetAccountSummaryInput{}).
		Return(&iam.GetAccountSummaryOutput{
			SummaryMap: map[string]int32{
				"Roles":      5,
				"RolesQuota": 1000,
			},
		}, nil)

	summary, err := getIAMAccountSummary(ctx, mockClient)

	assert.NoError(t, err)
	assert.NotNil(t, summary)
	assert.Equal(t, int32(5), summary.RoleCount, "Role count should be 5")
	assert.Equal(t, int32(1000), summary.RoleQuota, "Role quota should be 1000")
}

func TestGetIAMAccountSummary_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)
	ctx := context.TODO()

	mockClient.EXPECT().
		GetAccountSummary(ctx, &iam.GetAccountSummaryInput{}).
		Return(nil, assert.AnError)

	summary, err := getIAMAccountSummary(ctx, mockClient)

	assert.Error(t, err)
	assert.NotNil(t, summary, "Summary should not be nil even on error")
	assert.Equal(t, int32(0), summary.RoleCount, "Role count should be 0 on error")
	assert.Equal(t, int32(0), summary.RoleQuota, "Role quota should be 0 on error")
	assert.Contains(t, err.Error(), "assert.AnError")
}
