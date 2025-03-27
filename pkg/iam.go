package pkg

import (
	"context"
	"time"

	"github.com/app-sre/aws-resource-exporter/pkg/awsclient"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

type IAMClient interface {
	ListRolesPagesWithContext(ctx aws.Context, input *iam.ListRolesInput, fn func(*iam.ListRolesOutput, bool) bool, opts ...request.Option) error
}

var (
	IamRolesUsed = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "iam", "roles_used"),
		"Number of IAM roles used in the account.",
		[]string{"aws_account_id"}, nil,
	)
	IamRolesQuota = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "iam", "roles_quota"),
		"IAM role quota for the account.",
		[]string{"aws_account_id"}, nil,
	)
)

type IAMExporter struct {
	session      *session.Session
	iamClient    IAMClient
	sqClient     awsclient.Client
	logger       log.Logger
	timeout      time.Duration
	interval     time.Duration
	awsAccountId string
	cache        MetricsCache
}

// NewIAMExporter creates a new IAMExporter
func NewIAMExporter(sess *session.Session, logger log.Logger, config IAMConfig, awsAccountId string) *IAMExporter {
	level.Info(logger).Log("msg", "Initializing IAM exporter")

	return &IAMExporter{
		session:      sess,
		iamClient:    iam.New(sess),
		sqClient:     awsclient.NewClientFromSession(sess),
		logger:       logger,
		timeout:      *config.Timeout,
		interval:     *config.Interval,
		awsAccountId: awsAccountId,
		cache:        *NewMetricsCache(*config.CacheTTL),
	}
}

func (e *IAMExporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- IamRolesUsed
	ch <- IamRolesQuota
}

func (e *IAMExporter) Collect(ch chan<- prometheus.Metric) {
	for _, m := range e.cache.GetAllMetrics() {
		ch <- m
	}
}

func (e *IAMExporter) CollectLoop() {
	for {
		ctx, cancel := context.WithTimeout(context.Background(), e.timeout)

		roleCount, err := getIAMRoleCount(ctx, e.iamClient)
		if err != nil {
			level.Error(e.logger).Log("msg", "Failed to get IAM role count", "err", err)
			cancel()
			time.Sleep(e.interval)
			continue
		}

		quota, err := getQuotaValueWithContext(e.sqClient, "iam", "L-FE177D64", ctx)
		if err != nil {
			level.Error(e.logger).Log("msg", "Failed to get IAM role quota", "err", err)
			cancel()
			time.Sleep(e.interval)
			continue
		}

		e.cache.AddMetric(prometheus.MustNewConstMetric(IamRolesUsed, prometheus.GaugeValue, float64(roleCount), e.awsAccountId))
		e.cache.AddMetric(prometheus.MustNewConstMetric(IamRolesQuota, prometheus.GaugeValue, quota, e.awsAccountId))

		level.Info(e.logger).Log("msg", "IAM metrics updated", "used", roleCount, "quota", quota)
		cancel()
		time.Sleep(e.interval)
	}
}

// getIAMRoleCount returns number of IAM roles using IAMClient
func getIAMRoleCount(ctx context.Context, client IAMClient) (int, error) {
	var count int
	err := client.ListRolesPagesWithContext(ctx, &iam.ListRolesInput{
		MaxItems: aws.Int64(1000),
	}, func(output *iam.ListRolesOutput, _ bool) bool {
		count += len(output.Roles)
		return true
	})
	return count, err
}
