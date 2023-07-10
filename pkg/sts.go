package pkg

import (
	"os"
	"time"

	"github.com/app-sre/aws-resource-exporter/pkg/awsclient"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/go-kit/kit/log"
)

func AssumeRole(client awsclient.Sts, roleARN, sessionName string, durationInSeconds int64, logger log.Logger) error {
	roleInput := sts.AssumeRoleInput{
		RoleArn:         &roleARN,
		RoleSessionName: &sessionName,
		DurationSeconds: &durationInSeconds,
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

func RefreshToken(client awsclient.Sts, credentials *sts.Credentials, roleARN, sessionName string, durationInSeconds int64, logger log.Logger) error {
	for {
		expiration := credentials.Expiration
		refreshWindow := time.Minute * 5
		if expiration != nil && expiration.Sub(time.Now()) < refreshWindow {
			err := AssumeRole(client, roleARN, sessionName, durationInSeconds, logger)
			if err != nil {
				return err
			}
		}
	}
}
