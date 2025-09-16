package pkg

import (
	"context"
	"testing"
	"time"

	"github.com/app-sre/aws-resource-exporter/pkg/awsclient/mock"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iam_types "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestGetIAMRoleCount_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)

	// Mock a single page response with 3 roles
	mockClient.EXPECT().
		ListRoles(gomock.Any(), gomock.Any()).
		Return(&iam.ListRolesOutput{
			Roles: []iam_types.Role{{}, {}, {}},
			IsTruncated: false,
		}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	count, err := getIAMRoleCount(ctx, mockClient)
	assert.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestGetIAMRoleCount_Pagination(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockClient(ctrl)

	// Mock first page with 2 roles and truncated
	mockClient.EXPECT().
		ListRoles(gomock.Any(), gomock.Any()).
		Return(&iam.ListRolesOutput{
			Roles: []iam_types.Role{{}, {}},
			IsTruncated: true,
			Marker: aws.String("marker1"),
		}, nil)

	// Mock second page with 1 role and not truncated
	mockClient.EXPECT().
		ListRoles(gomock.Any(), gomock.Any()).
		Return(&iam.ListRolesOutput{
			Roles: []iam_types.Role{{}},
			IsTruncated: false,
		}, nil)

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
		ListRoles(gomock.Any(), gomock.Any()).
		Return(nil, assert.AnError)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	count, err := getIAMRoleCount(ctx, mockClient)
	assert.Error(t, err)
	assert.Equal(t, 0, count)
	assert.Contains(t, err.Error(), "assert.AnError")
}
