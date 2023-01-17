package pkg

import (
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

// func TakeRole(sess *session.Session, roleARN, sessionName string) (*sts.AssumeRoleOutput, error) {
func TakeRole(sess *session.Session, roleARN, sessionName string, logger log.Logger) {
	stsClient := sts.New(sess)

	result, err := stsClient.AssumeRole(&sts.AssumeRoleInput{
		RoleArn:         &roleARN,
		RoleSessionName: &sessionName,
	})
	if err != nil {
		level.Error(logger).Log("msg", "Could not assume VPC read only role", "err", err)
	}

	if err := os.Setenv("AWS_ACCESS_KEY_ID", *result.Credentials.AccessKeyId); err != nil {
		fmt.Printf("Couldn't set AWS_ACCESS_KEY_ID environment variable. Err: %v", err)
	}

	if err := os.Setenv("AWS_SECRET_ACCESS_KEY", *result.Credentials.SecretAccessKey); err != nil {
		fmt.Printf("Couldn't set AWS_SECRET_ACCESS_KEY environment variable. Err: %v", err)
	}

	if err := os.Setenv("AWS_SESSION_TOKEN", *result.Credentials.SessionToken); err != nil {
		fmt.Printf("Couldn't set AWS_SESSION_TOKEN environment variable. Err: %v", err)
	}

	return
}
