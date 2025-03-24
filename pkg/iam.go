package pkg

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/servicequotas"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

type IAMClient interface {
	ListRolesPagesWithContext(ctx aws.Context, input *iam.ListRolesInput, fn func(*iam.ListRolesOutput, bool) bool, opts ...request.Option) error
}

type ServiceQuotasClient interface {
	GetServiceQuotaWithContext(ctx aws.Context, input *servicequotas.GetServiceQuotaInput, opts ...request.Option) (*servicequotas.GetServiceQuotaOutput, error)
}

type AWSIAMClient struct {
	iam *iam.IAM
}

func (c *AWSIAMClient) ListRolesPagesWithContext(ctx aws.Context, input *iam.ListRolesInput, fn func(*iam.ListRolesOutput, bool) bool, opts ...request.Option) error {
	return c.iam.ListRolesPagesWithContext(ctx, input, fn, opts...)
}

type AWSServiceQuotasClient struct {
	sq *servicequotas.ServiceQuotas
}

func (c *AWSServiceQuotasClient) GetServiceQuotaWithContext(ctx aws.Context, input *servicequotas.GetServiceQuotaInput, opts ...request.Option) (*servicequotas.GetServiceQuotaOutput, error) {
	return c.sq.GetServiceQuotaWithContext(ctx, input, opts...)
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
	iamClient    IAMClient
	sqClient     ServiceQuotasClient
	logger       log.Logger
	timeout      time.Duration
	interval     time.Duration
	awsAccountId string
}

func NewIAMExporter(sess *session.Session, logger log.Logger, config IAMConfig, awsAccountId string) *IAMExporter {
	level.Info(logger).Log("msg", "Initializing IAM exporter")

	return &IAMExporter{
		iamClient:    &AWSIAMClient{iam: iam.New(sess)},
		sqClient:     &AWSServiceQuotasClient{sq: servicequotas.New(sess)},
		logger:       logger,
		timeout:      *config.Timeout,
		interval:     *config.Interval,
		awsAccountId: awsAccountId,
	}
}

func (e *IAMExporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- IamRolesUsed
	ch <- IamRolesQuota
}

func (e *IAMExporter) Collect(ch chan<- prometheus.Metric) {
	used, quota, _, err := e.getIAMMetrics()
	if err != nil {
		level.Error(e.logger).Log("msg", "Failed to get IAM metrics", "err", err)
		return
	}

	ch <- prometheus.MustNewConstMetric(IamRolesUsed, prometheus.GaugeValue, float64(used), e.awsAccountId)
	ch <- prometheus.MustNewConstMetric(IamRolesQuota, prometheus.GaugeValue, quota, e.awsAccountId)
}

func (e *IAMExporter) getIAMMetrics() (int, float64, float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	defer cancel()

	var roleCount int
	err := e.iamClient.ListRolesPagesWithContext(ctx, &iam.ListRolesInput{
		MaxItems: aws.Int64(1000),
	}, func(output *iam.ListRolesOutput, _ bool) bool {
		roleCount += len(output.Roles)
		return true
	})

	quotaResp, quotaErr := e.sqClient.GetServiceQuotaWithContext(ctx, &servicequotas.GetServiceQuotaInput{
		ServiceCode: aws.String("iam"),
		QuotaCode:   aws.String("L-FE177D64"),
	})

	roleQuota := 0.0
	if quotaErr == nil && quotaResp.Quota.Value != nil {
		roleQuota = *quotaResp.Quota.Value
	}

	usagePercent := 0.0
	if roleQuota > 0 {
		usagePercent = (float64(roleCount) / roleQuota) * 100
	}

	if err != nil {
		return 0, roleQuota, usagePercent, fmt.Errorf("error listing IAM roles: %w", err)
	}

	return roleCount, roleQuota, usagePercent, nil
}

func (e *IAMExporter) CollectLoop() {
	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			used, quota, usagePercent, err := e.getIAMMetrics()
			if err != nil {
				level.Error(e.logger).Log("msg", "Error collecting IAM metrics", "err", err)
			} else {
				level.Info(e.logger).Log("msg", "IAM metrics collected", "used", used, "quota", quota, "usage_percent", usagePercent)
			}
		}
	}
}
