package pkg

import (
	"context"
	"testing"
	"time"

	"github.com/app-sre/aws-resource-exporter/pkg/awsclient/mock"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestGetIAMRoleCount_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIAM := mock.NewMockIAMClient(ctrl)
	mockIAM.EXPECT().
		ListRolesPagesWithContext(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx aws.Context, input *iam.ListRolesInput, fn func(*iam.ListRolesOutput, bool) bool, opts ...interface{}) error {
			fn(&iam.ListRolesOutput{Roles: []*iam.Role{{}, {}, {}}}, true)
			return nil
		})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	count, err := getIAMRoleCount(ctx, mockIAM)
	assert.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestGetIAMRoleCount_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIAM := mock.NewMockIAMClient(ctrl)
	mockIAM.EXPECT().
		ListRolesPagesWithContext(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(assert.AnError)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	count, err := getIAMRoleCount(ctx, mockIAM)
	assert.Error(t, err)
	assert.Equal(t, 0, count)
	assert.Contains(t, err.Error(), "assert.AnError")
}
