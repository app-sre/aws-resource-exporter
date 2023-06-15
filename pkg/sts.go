package pkg

import (
	"os"

	"github.com/app-sre/aws-resource-exporter/pkg/awsclient"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/go-kit/kit/log"
)

func AssumeRole(client awsclient.Sts, roleARN, sessionName string, logger log.Logger) error {
	roleInput := sts.AssumeRoleInput{
		RoleArn:         &roleARN,
		RoleSessionName: &sessionName,
	}

	result, err := client.AssumeRole(&roleInput)
	if err != nil {
		return err
	}

	if err := os.Setenv("AWS_ACCESS_KEY_ID", *result.Credentials.AccessKeyId); err != nil {
		return err
	}

	if err := os.Setenv("AWS_SECRET_ACCESS_KEY", *result.Credentials.SecretAccessKey); err != nil {
		return err
	}

	if err := os.Setenv("AWS_SESSION_TOKEN", *result.Credentials.SessionToken); err != nil {
		return err
	}
	return nil
}

func LookUpEnvVar(key string) bool {
	_, ok := os.LookupEnv(key)
	return ok
}
