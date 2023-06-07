package pkg

import (
	"os"
	"testing"

	"github.com/app-sre/aws-resource-exporter/pkg/awsclient/mock"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestTakeRoleSTS(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := mock.NewMockClient(ctrl)

	assumeRoleInput := &sts.AssumeRoleInput{
		RoleArn:         aws.String("arn:aws:iam::123456789012:role/example"),
		RoleSessionName: aws.String("session-name"),
	}
	assumeRoleOutput := &sts.AssumeRoleOutput{
		Credentials: &sts.Credentials{
			AccessKeyId:     aws.String("access-key-id"),
			SecretAccessKey: aws.String("secret-access-key"),
			SessionToken:    aws.String("session-token"),
		},
	}
	mockSvc.EXPECT().AssumeRole(assumeRoleInput).Return(assumeRoleOutput, nil)
	err := AssumeRole(mockSvc, "arn:aws:iam::123456789012:role/example", "session-name", nil)
	// assert.Equal(t, *assumeRoleResult.Credentials.AccessKeyId, "access-key-id")
	// assert.Equal(t, *assumeRoleResult.Credentials.SecretAccessKey, "secret-access-key")
	// assert.Equal(t, *assumeRoleResult.Credentials.SessionToken, "session-token")
	assert.NoError(t, err)
	assert.Equal(t, "access-key-id", os.Getenv("AWS_ACCESS_KEY_ID"))
	assert.Equal(t, "secret-access-key", os.Getenv("AWS_SECRET_ACCESS_KEY"))
	assert.Equal(t, "session-token", os.Getenv("AWS_SESSION_TOKEN"))
}
