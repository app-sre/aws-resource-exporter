package pkg

import (
	"context"
	"testing"
	"time"

	"github.com/app-sre/aws-resource-exporter/pkg/awsclient/mock"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestGetIAMRoleCount_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIAM := mock.NewMockIAMClient(ctrl)
	mockIAM.EXPECT().
		ListRoles(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&iam.ListRolesOutput{Roles: []types.Role{{}, {}, {}}}, nil)

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
		ListRoles(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, assert.AnError)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	count, err := getIAMRoleCount(ctx, mockIAM)
	assert.Error(t, err)
	assert.Equal(t, 0, count)
	assert.Contains(t, err.Error(), "assert.AnError")
}
