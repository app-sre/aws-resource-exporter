package pkg

import (
	"context"
	"sync"
	"time"

	"github.com/app-sre/aws-resource-exporter/pkg/awsclient"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/servicequotas"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	QUOTA_VPCS_PER_REGION                 string = "L-F678F1CE"
	QUOTA_SUBNETS_PER_VPC                 string = "L-407747CB"
	QUOTA_ROUTES_PER_ROUTE_TABLE          string = "L-93826ACB"
	QUOTA_INTERFACE_VPC_ENDPOINTS_PER_VPC string = "L-29B6F2EB"
	QUOTA_ROUTE_TABLES_PER_VPC            string = "L-589F43AA"
	QUOTA_IPV4_BLOCKS_PER_VPC             string = "L-83CA0A9D"
	SERVICE_CODE_VPC                      string = "vpc"
)

type VPCExporter struct {
	awsAccountId                     string
	sessions                         []*session.Session
	VpcsPerRegionQuota               *prometheus.Desc
	VpcsPerRegionUsage               *prometheus.Desc
	SubnetsPerVpcQuota               *prometheus.Desc
	SubnetsPerVpcUsage               *prometheus.Desc
	RoutesPerRouteTableQuota         *prometheus.Desc
	RoutesPerRouteTableUsage         *prometheus.Desc
	InterfaceVpcEndpointsPerVpcQuota *prometheus.Desc
	InterfaceVpcEndpointsPerVpcUsage *prometheus.Desc
	RouteTablesPerVpcQuota           *prometheus.Desc
	RouteTablesPerVpcUsage           *prometheus.Desc
	IPv4BlocksPerVpcQuota            *prometheus.Desc
	IPv4BlocksPerVpcUsage            *prometheus.Desc

	logger   log.Logger
	timeout  time.Duration
	cache    MetricsCache
	interval time.Duration
}

type VPCCollector struct {
	e             *VPCExporter
	ec2           *ec2.EC2
	serviceQuotas *servicequotas.ServiceQuotas
	region        *string
	wg            *sync.WaitGroup
}

func NewVPCExporter(sess []*session.Session, logger log.Logger, config VPCConfig, awsAccountId string) *VPCExporter {
	level.Info(logger).Log("msg", "Initializing VPC exporter")
	constLabels := map[string]string{"aws_account_id": awsAccountId, SERVICE_CODE_KEY: SERVICE_CODE_VPC}
	return &VPCExporter{
		awsAccountId:                     awsAccountId,
		sessions:                         sess,
		VpcsPerRegionQuota:               prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "vpc_vpcsperregion_quota"), "The quota of VPCs per region", []string{"aws_region"}, WithKeyValue(constLabels, QUOTA_CODE_KEY, QUOTA_VPCS_PER_REGION)),
		VpcsPerRegionUsage:               prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "vpc_vpcsperregion_usage"), "The usage of VPCs per region", []string{"aws_region"}, WithKeyValue(constLabels, QUOTA_CODE_KEY, QUOTA_VPCS_PER_REGION)),
		SubnetsPerVpcQuota:               prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "vpc_subnetspervpc_quota"), "The quota of subnets per VPC", []string{"aws_region"}, WithKeyValue(constLabels, QUOTA_CODE_KEY, QUOTA_SUBNETS_PER_VPC)),
		SubnetsPerVpcUsage:               prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "vpc_subnetspervpc_usage"), "The usage of subnets per VPC", []string{"aws_region", "vpcid"}, WithKeyValue(constLabels, QUOTA_CODE_KEY, QUOTA_SUBNETS_PER_VPC)),
		RoutesPerRouteTableQuota:         prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "vpc_routesperroutetable_quota"), "The quota of routes per routetable", []string{"aws_region"}, WithKeyValue(constLabels, QUOTA_CODE_KEY, QUOTA_ROUTES_PER_ROUTE_TABLE)),
		RoutesPerRouteTableUsage:         prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "vpc_routesperroutetable_usage"), "The usage of routes per routetable", []string{"aws_region", "vpcid", "routetableid"}, WithKeyValue(constLabels, QUOTA_CODE_KEY, QUOTA_ROUTES_PER_ROUTE_TABLE)),
		InterfaceVpcEndpointsPerVpcQuota: prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "vpc_interfacevpcendpointspervpc_quota"), "The quota of interface vpc endpoints per vpc", []string{"aws_region"}, WithKeyValue(constLabels, QUOTA_CODE_KEY, QUOTA_INTERFACE_VPC_ENDPOINTS_PER_VPC)),
		InterfaceVpcEndpointsPerVpcUsage: prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "vpc_interfacevpcendpointspervpc_usage"), "The usage of interface vpc endpoints per vpc", []string{"aws_region", "vpcid"}, WithKeyValue(constLabels, QUOTA_CODE_KEY, QUOTA_INTERFACE_VPC_ENDPOINTS_PER_VPC)),
		RouteTablesPerVpcQuota:           prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "vpc_routetablespervpc_quota"), "The quota of route tables per vpc", []string{"aws_region"}, WithKeyValue(constLabels, QUOTA_CODE_KEY, QUOTA_ROUTE_TABLES_PER_VPC)),
		RouteTablesPerVpcUsage:           prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "vpc_routetablespervpc_usage"), "The usage of route tables per vpc", []string{"aws_region", "vpcid"}, WithKeyValue(constLabels, QUOTA_CODE_KEY, QUOTA_ROUTE_TABLES_PER_VPC)),
		IPv4BlocksPerVpcQuota:            prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "vpc_ipv4blockspervpc_quota"), "The quota of ipv4 blocks per vpc", []string{"aws_region"}, WithKeyValue(constLabels, QUOTA_CODE_KEY, QUOTA_IPV4_BLOCKS_PER_VPC)),
		IPv4BlocksPerVpcUsage:            prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "vpc_ipv4blockspervpc_usage"), "The usage of ipv4 blocks per vpc", []string{"aws_region", "vpcid"}, WithKeyValue(constLabels, QUOTA_CODE_KEY, QUOTA_IPV4_BLOCKS_PER_VPC)),
		logger:                           logger,
		timeout:                          *config.Timeout,
		cache:                            *NewMetricsCache(*config.CacheTTL),
		interval:                         *config.Interval,
	}
}

func (e *VPCExporter) CollectInRegion(session *session.Session, region *string, wg *sync.WaitGroup) {
	defer wg.Done()

	awsClient := awsclient.NewClientFromSession(session)

	e.collectVpcsPerRegionQuota(awsClient, *region)
	e.collectVpcsPerRegionUsage(awsClient, *region)
	e.collectRoutesTablesPerVpcQuota(awsClient, *region)
	e.collectInterfaceVpcEndpointsPerVpcQuota(awsClient, *region)
	e.collectSubnetsPerVpcQuota(awsClient, *region)
	e.collectIPv4BlocksPerVpcQuota(awsClient, *region)
	vpcCtx, vpcCancel := context.WithTimeout(context.Background(), e.timeout)
	defer vpcCancel()
	var allVpcs []*ec2.Vpc
	err := awsClient.DescribeVpcsPagesWithContext(vpcCtx, &ec2.DescribeVpcsInput{}, func(out *ec2.DescribeVpcsOutput, lastPage bool) bool {
		allVpcs = append(allVpcs, out.Vpcs...)
		return !lastPage
	})
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to DescribeVpcs failed", "region", region, "err", err)
	} else {
		for i := range allVpcs {
			e.collectSubnetsPerVpcUsage(allVpcs[i], awsClient, *region)
			e.collectInterfaceVpcEndpointsPerVpcUsage(allVpcs[i], awsClient, *region)
			e.collectRoutesTablesPerVpcUsage(allVpcs[i], awsClient, *region)
			e.collectIPv4BlocksPerVpcUsage(allVpcs[i], awsClient, *region)
		}
	}
	e.collectRoutesPerRouteTableQuota(awsClient, *region)
	routesCtx, routesCancel := context.WithTimeout(context.Background(), e.timeout)
	defer routesCancel()
	var allRouteTables []*ec2.RouteTable
	err = awsClient.DescribeRouteTablesPagesWithContext(routesCtx, &ec2.DescribeRouteTablesInput{}, func(out *ec2.DescribeRouteTablesOutput, lastPage bool) bool {
		allRouteTables = append(allRouteTables, out.RouteTables...)
		return !lastPage
	})
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to DescribeRouteTables failed", "region", region, "err", err)
	} else {
		for i := range allRouteTables {
			e.collectRoutesPerRouteTableUsage(allRouteTables[i], awsClient, *region)
		}
	}
}

func (e *VPCExporter) CollectLoop() {
	for {

		wg := &sync.WaitGroup{}
		wg.Add(len(e.sessions))
		for i := range e.sessions {
			session := e.sessions[i]
			region := session.Config.Region
			go e.CollectInRegion(session, region, wg)
		}
		wg.Wait()

		level.Info(e.logger).Log("msg", "VPC metrics Updated")

		time.Sleep(e.interval)
	}
}

func (e *VPCExporter) Collect(ch chan<- prometheus.Metric) {
	for _, m := range e.cache.GetAllMetrics() {
		ch <- m
	}
}

func (e *VPCExporter) GetQuotaValue(client awsclient.Client, serviceCode string, quotaCode string) (float64, error) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), e.timeout)
	defer cancelFunc()
	sqOutput, err := client.GetServiceQuotaWithContext(ctx, &servicequotas.GetServiceQuotaInput{
		QuotaCode:   aws.String(quotaCode),
		ServiceCode: aws.String(serviceCode),
	})

	if err != nil {
		return 0, err
	}

	return *sqOutput.Quota.Value, nil
}

func (e *VPCExporter) collectVpcsPerRegionQuota(client awsclient.Client, region string) {
	quota, err := e.GetQuotaValue(client, SERVICE_CODE_VPC, QUOTA_VPCS_PER_REGION)
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to VpcsPerRegion ServiceQuota failed", "region", region, "err", err)
		awsclient.AwsExporterMetrics.IncrementErrors()
		return
	}
	e.cache.AddMetric(prometheus.MustNewConstMetric(e.VpcsPerRegionQuota, prometheus.GaugeValue, quota, region))
}

func (e *VPCExporter) collectVpcsPerRegionUsage(client awsclient.Client, region string) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), e.timeout)
	defer cancelFunc()
	numVpcs := 0
	err := client.DescribeVpcsPagesWithContext(ctx, &ec2.DescribeVpcsInput{}, func(page *ec2.DescribeVpcsOutput, lastPage bool) bool {
		numVpcs += len(page.Vpcs)
		return !lastPage
	})
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to DescribeVpcs failed", "region", region, "err", err)
		awsclient.AwsExporterMetrics.IncrementErrors()
		return
	}
	e.cache.AddMetric(prometheus.MustNewConstMetric(e.VpcsPerRegionUsage, prometheus.GaugeValue, float64(numVpcs), region))
}

func (e *VPCExporter) collectSubnetsPerVpcQuota(client awsclient.Client, region string) {
	quota, err := e.GetQuotaValue(client, SERVICE_CODE_VPC, QUOTA_SUBNETS_PER_VPC)
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to SubnetsPerVpc ServiceQuota failed", "region", region, "err", err)
		awsclient.AwsExporterMetrics.IncrementErrors()
		return
	}
	e.cache.AddMetric(prometheus.MustNewConstMetric(e.SubnetsPerVpcQuota, prometheus.GaugeValue, quota, region))
}

func (e *VPCExporter) collectSubnetsPerVpcUsage(vpc *ec2.Vpc, client awsclient.Client, region string) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), e.timeout)
	defer cancelFunc()
	numSubnets := 0
	err := client.DescribeSubnetsPagesWithContext(ctx, &ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{{
			Name:   aws.String("vpc-id"),
			Values: []*string{vpc.VpcId},
		}}}, func(page *ec2.DescribeSubnetsOutput, lastPage bool) bool {
		numSubnets += len(page.Subnets)
		return !lastPage
	})
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to DescribeSubnets failed", "region", region, "err", err)
		awsclient.AwsExporterMetrics.IncrementErrors()
		return
	}
	e.cache.AddMetric(prometheus.MustNewConstMetric(e.SubnetsPerVpcUsage, prometheus.GaugeValue, float64(numSubnets), region, *vpc.VpcId))
}

func (e *VPCExporter) collectRoutesPerRouteTableQuota(client awsclient.Client, region string) {
	quota, err := e.GetQuotaValue(client, SERVICE_CODE_VPC, QUOTA_ROUTES_PER_ROUTE_TABLE)
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to RoutesPerRouteTable ServiceQuota failed", "region", region, "err", err)
		awsclient.AwsExporterMetrics.IncrementErrors()
		return
	}
	e.cache.AddMetric(prometheus.MustNewConstMetric(e.RoutesPerRouteTableQuota, prometheus.GaugeValue, quota, region))
}

func (e *VPCExporter) collectRoutesPerRouteTableUsage(rtb *ec2.RouteTable, client awsclient.Client, region string) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), e.timeout)
	defer cancelFunc()
	descRouteTableOutput, err := client.DescribeRouteTablesWithContext(ctx, &ec2.DescribeRouteTablesInput{
		RouteTableIds: []*string{rtb.RouteTableId},
	})
	if len(descRouteTableOutput.RouteTables) != 1 {
		level.Error(e.logger).Log("msg", "Unexpected number of routetables (!= 1) returned from DescribeRouteTables")
		return
	}
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to DescribeRouteTables failed", "region", region, "err", err)
		awsclient.AwsExporterMetrics.IncrementErrors()
		return
	}
	quota := len(descRouteTableOutput.RouteTables)
	e.cache.AddMetric(prometheus.MustNewConstMetric(e.RoutesPerRouteTableUsage, prometheus.GaugeValue, float64(quota), region, *rtb.VpcId, *rtb.RouteTableId))
}

func (e *VPCExporter) collectInterfaceVpcEndpointsPerVpcQuota(client awsclient.Client, region string) {
	quota, err := e.GetQuotaValue(client, SERVICE_CODE_VPC, QUOTA_INTERFACE_VPC_ENDPOINTS_PER_VPC)
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to InterfaceVpcEndpointsPerVpc ServiceQuota failed", "region", region, "err", err)
		awsclient.AwsExporterMetrics.IncrementErrors()
		return
	}
	e.cache.AddMetric(prometheus.MustNewConstMetric(e.InterfaceVpcEndpointsPerVpcQuota, prometheus.GaugeValue, quota, region))
}

func (e *VPCExporter) collectInterfaceVpcEndpointsPerVpcUsage(vpc *ec2.Vpc, client awsclient.Client, region string) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), e.timeout)
	defer cancelFunc()

	numEndpoints := 0
	descEndpointsInput := &ec2.DescribeVpcEndpointsInput{
		Filters: []*ec2.Filter{{Name: aws.String("vpc-id"), Values: []*string{vpc.VpcId}}},
	}
	err := client.DescribeVpcEndpointsPagesWithContext(ctx, descEndpointsInput, func(page *ec2.DescribeVpcEndpointsOutput, lastPage bool) bool {
		numEndpoints += len(page.VpcEndpoints)
		return !lastPage
	})
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to DescribeVpcEndpoints failed", "region", region, "err", err)
		awsclient.AwsExporterMetrics.IncrementErrors()
		return
	}
	e.cache.AddMetric(prometheus.MustNewConstMetric(e.InterfaceVpcEndpointsPerVpcUsage, prometheus.GaugeValue, float64(numEndpoints), region, *vpc.VpcId))
}

func (e *VPCExporter) collectRoutesTablesPerVpcQuota(client awsclient.Client, region string) {
	quota, err := e.GetQuotaValue(client, SERVICE_CODE_VPC, QUOTA_ROUTE_TABLES_PER_VPC)
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to RoutesTablesPerVpc ServiceQuota failed", "region", region, "err", err)
		awsclient.AwsExporterMetrics.IncrementErrors()
		return
	}
	e.cache.AddMetric(prometheus.MustNewConstMetric(e.RouteTablesPerVpcQuota, prometheus.GaugeValue, quota, region))
}

func (e *VPCExporter) collectRoutesTablesPerVpcUsage(vpc *ec2.Vpc, client awsclient.Client, region string) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), e.timeout)
	defer cancelFunc()
	var numRouteTables int
	input := &ec2.DescribeRouteTablesInput{
		Filters: []*ec2.Filter{{
			Name:   aws.String("vpc-id"),
			Values: []*string{vpc.VpcId},
		}}}
	err := client.DescribeRouteTablesPagesWithContext(ctx, input, func(page *ec2.DescribeRouteTablesOutput, lastPage bool) bool {
		numRouteTables += len(page.RouteTables)
		return !lastPage
	})
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to DescribeRouteTables failed", "region", region, "err", err)
		awsclient.AwsExporterMetrics.IncrementErrors()
		return
	}
	e.cache.AddMetric(prometheus.MustNewConstMetric(e.RouteTablesPerVpcUsage, prometheus.GaugeValue, float64(numRouteTables), region, *vpc.VpcId))
}

func (e *VPCExporter) collectIPv4BlocksPerVpcQuota(client awsclient.Client, region string) {
	quota, err := e.GetQuotaValue(client, SERVICE_CODE_VPC, QUOTA_IPV4_BLOCKS_PER_VPC)
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to IPv4BlocksPerVpc ServiceQuota failed", "region", region, "err", err)
		awsclient.AwsExporterMetrics.IncrementErrors()
		return
	}
	e.cache.AddMetric(prometheus.MustNewConstMetric(e.IPv4BlocksPerVpcQuota, prometheus.GaugeValue, quota, region))
}

func (e *VPCExporter) collectIPv4BlocksPerVpcUsage(vpc *ec2.Vpc, client awsclient.Client, region string) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), e.timeout)
	defer cancelFunc()
	descVpcs, err := client.DescribeVpcsWithContext(ctx, &ec2.DescribeVpcsInput{
		VpcIds: []*string{vpc.VpcId},
	})
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to DescribeVpcs failed", "region", region, "err", err)
		awsclient.AwsExporterMetrics.IncrementErrors()
		return
	}
	if len(descVpcs.Vpcs) != 1 {
		level.Error(e.logger).Log("msg", "Unexpected numbers of VPCs (!= 1) returned", "region", region, "vpcId", vpc.VpcId)
	}
	quota := len(descVpcs.Vpcs[0].CidrBlockAssociationSet)
	e.cache.AddMetric(prometheus.MustNewConstMetric(e.IPv4BlocksPerVpcUsage, prometheus.GaugeValue, float64(quota), region, *vpc.VpcId))
}

func (e *VPCExporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.VpcsPerRegionQuota
	ch <- e.VpcsPerRegionUsage
	ch <- e.SubnetsPerVpcQuota
	ch <- e.SubnetsPerVpcUsage
	ch <- e.RoutesPerRouteTableQuota
	ch <- e.RoutesPerRouteTableUsage
	ch <- e.IPv4BlocksPerVpcQuota
	ch <- e.IPv4BlocksPerVpcUsage
	ch <- e.InterfaceVpcEndpointsPerVpcQuota
	ch <- e.InterfaceVpcEndpointsPerVpcUsage
	ch <- e.RouteTablesPerVpcQuota
	ch <- e.RoutesPerRouteTableUsage
}
