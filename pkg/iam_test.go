package pkg

import (
	"context"
	"testing"
	"time"

	"github.com/app-sre/aws-resource-exporter/pkg/awsclient/mock"
	iam_types "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestGetIAMRoleCount_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)
	mockClient.EXPECT().
		ListRolesAll(gomock.Any()).
		Return([]iam_types.Role{{}, {}, {}}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	count, err := getIAMRoleCount(ctx, mockClient)
	assert.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestGetIAMRoleCount_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)
	mockClient.EXPECT().
		ListRolesAll(gomock.Any()).
		Return(nil, assert.AnError)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	count, err := getIAMRoleCount(ctx, mockClient)
	assert.Error(t, err)
	assert.Equal(t, 0, count)
	assert.Contains(t, err.Error(), "assert.AnError")
}
