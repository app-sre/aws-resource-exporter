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

type AccountSummary struct {
	RoleCount int32
	RoleQuota int32
}

type IAMExporter struct {
	config       aws.Config
	iamClient    awsclient.Client
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
		config:       cfg,
		iamClient:    awsclient.NewClientFromConfig(cfg),
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

		summary, err := getIAMAccountSummary(ctx, e.iamClient)

		if err != nil {
			e.logger.Error("Failed to get IAM account summary", slog.Any("err", err))
			cancel()
			time.Sleep(e.interval)
			continue
		}

		e.cache.AddMetric(prometheus.MustNewConstMetric(IamRolesUsed, prometheus.GaugeValue, float64(summary.RoleCount), e.awsAccountId))
		e.cache.AddMetric(prometheus.MustNewConstMetric(IamRolesQuota, prometheus.GaugeValue, float64(summary.RoleQuota), e.awsAccountId))

		e.logger.Info("IAM metrics updated",
			slog.Int("used", int(summary.RoleCount)),
			slog.Int("quota", int(summary.RoleQuota)))

		cancel()
		time.Sleep(e.interval)
	}
}

func getIAMAccountSummary(ctx context.Context, client awsclient.Client) (*AccountSummary, error) {
	accountSummary := &AccountSummary{
		RoleCount: 0,
		RoleQuota: 0,
	}

	summary, err := client.GetAccountSummary(ctx, &iam.GetAccountSummaryInput{})
	if err != nil {
		return accountSummary, err
	}

	if val, exists := summary.SummaryMap["Roles"]; exists {
		accountSummary.RoleCount = val
	}
	if val, exists := summary.SummaryMap["RolesQuota"]; exists {
		accountSummary.RoleQuota = val
	}

	return accountSummary, nil
}
