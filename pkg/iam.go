package pkg

import (
	"context"
	"log/slog"
	"time"

	"github.com/app-sre/aws-resource-exporter/pkg/awsclient"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/prometheus/client_golang/prometheus"
)

//go:generate mockgen -source=./iam.go -destination=./awsclient/mock/mock_iam.go -package=mock
type IAMClient interface {
	ListRoles(ctx context.Context, input *iam.ListRolesInput, optFns ...func(*iam.Options)) (*iam.ListRolesOutput, error)
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
	cfg          aws.Config
	iamClient    IAMClient
	sqClient     awsclient.Client
	logger       *slog.Logger
	timeout      time.Duration
	interval     time.Duration
	awsAccountId string
	cache        MetricsCache
}

// NewIAMExporter creates a new IAMExporter
func NewIAMExporter(cfg aws.Config, logger *slog.Logger, config IAMConfig, awsAccountId string) *IAMExporter {
	logger.Info("Initializing IAM exporter")

	return &IAMExporter{
		cfg:          cfg,
		iamClient:    iam.NewFromConfig(cfg),
		sqClient:     awsclient.NewClientFromConfig(cfg),
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
			e.logger.Error("Failed to get IAM role count", slog.Any("err", err))
			cancel()
			time.Sleep(e.interval)
			continue
		}

		quota, err := getQuotaValue(e.sqClient, "iam", "L-FE177D64", ctx)
		if err != nil {
			e.logger.Error("Failed to get IAM role quota", slog.Any("err", err))
			cancel()
			time.Sleep(e.interval)
			continue
		}

		e.cache.AddMetric(prometheus.MustNewConstMetric(IamRolesUsed, prometheus.GaugeValue, float64(roleCount), e.awsAccountId))
		e.cache.AddMetric(prometheus.MustNewConstMetric(IamRolesQuota, prometheus.GaugeValue, quota, e.awsAccountId))

		e.logger.Info("IAM metrics updated",
			slog.Int("used", roleCount),
			slog.Float64("quota", quota))

		cancel()
		time.Sleep(e.interval)
	}
}

// getIAMRoleCount returns number of IAM roles using IAMClient
func getIAMRoleCount(ctx context.Context, client IAMClient) (int, error) {
	var count int
	paginator := iam.NewListRolesPaginator(client, &iam.ListRolesInput{
		MaxItems: aws.Int32(1000),
	})
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return 0, err
		}
		count += len(output.Roles)
	}
	return count, nil
}
