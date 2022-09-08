package main

import (
	"context"
	"sync"
	"time"

	"github.com/app-sre/aws-resource-exporter/pkg"
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
	cache    pkg.MetricsCache
	interval time.Duration
}

type VPCCollector struct {
	e             *VPCExporter
	ec2           *ec2.EC2
	serviceQuotas *servicequotas.ServiceQuotas
	region        *string
	wg            *sync.WaitGroup
}

func NewVPCExporter(sess []*session.Session, logger log.Logger, config VPCConfig) *VPCExporter {
	level.Info(logger).Log("msg", "Initializing VPC exporter")
	return &VPCExporter{
		sessions:                         sess,
		VpcsPerRegionQuota:               prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "vpc_vpcsperregion_quota"), "The quota of VPCs per region", []string{"aws_region"}, nil),
		VpcsPerRegionUsage:               prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "vpc_vpcsperregion_usage"), "The usage of VPCs per region", []string{"aws_region"}, nil),
		SubnetsPerVpcQuota:               prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "vpc_subnetspervpc_quota"), "The quota of subnets per VPC", []string{"aws_region"}, nil),
		SubnetsPerVpcUsage:               prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "vpc_subnetspervpc_usage"), "The usage of subnets per VPC", []string{"aws_region", "vpcid"}, nil),
		RoutesPerRouteTableQuota:         prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "vpc_routesperroutetable_quota"), "The quota of routes per routetable", []string{"aws_region"}, nil),
		RoutesPerRouteTableUsage:         prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "vpc_routesperroutetable_usage"), "The usage of routes per routetable", []string{"aws_region", "vpcid", "routetableid"}, nil),
		InterfaceVpcEndpointsPerVpcQuota: prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "vpc_interfacevpcendpointspervpc_quota"), "The quota of interface vpc endpoints per vpc", []string{"aws_region"}, nil),
		InterfaceVpcEndpointsPerVpcUsage: prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "vpc_interfacevpcendpointspervpc_usage"), "The usage of interface vpc endpoints per vpc", []string{"aws_region", "vpcid"}, nil),
		RouteTablesPerVpcQuota:           prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "vpc_routetablespervpc_quota"), "The quota of route tables per vpc", []string{"aws_region"}, nil),
		RouteTablesPerVpcUsage:           prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "vpc_routetablespervpc_usage"), "The usage of route tables per vpc", []string{"aws_region", "vpcid"}, nil),
		IPv4BlocksPerVpcQuota:            prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "vpc_ipv4blockspervpc_quota"), "The quota of ipv4 blocks per vpc", []string{"aws_region"}, nil),
		IPv4BlocksPerVpcUsage:            prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "vpc_ipv4blockspervpc_usage"), "The usage of ipv4 blocks per vpc", []string{"aws_region", "vpcid"}, nil),
		logger:                           logger,
		timeout:                          *config.Timeout,
		cache:                            *pkg.NewMetricsCache(*config.CacheTTL),
		interval:                         *config.Interval,
	}
}

func (e *VPCExporter) CollectInRegion(session *session.Session, region *string, wg *sync.WaitGroup) {
	defer wg.Done()

	ec2Svc := ec2.New(session)
	quotaSvc := servicequotas.New(session)

	e.collectVpcsPerRegionQuota(quotaSvc, *region)
	e.collectVpcsPerRegionUsage(ec2Svc, *region)
	e.collectRoutesTablesPerVpcQuota(quotaSvc, *region)
	e.collectInterfaceVpcEndpointsPerVpcQuota(quotaSvc, *region)
	e.collectSubnetsPerVpcQuota(quotaSvc, *region)
	e.collectIPv4BlocksPerVpcQuota(quotaSvc, *region)
	vpcCtx, vpcCancel := context.WithTimeout(context.Background(), e.timeout)
	defer vpcCancel()
	allVpcs, err := ec2Svc.DescribeVpcsWithContext(vpcCtx, &ec2.DescribeVpcsInput{})
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to DescribeVpcs failed", "region", region, "err", err)
	} else {
		for i, _ := range allVpcs.Vpcs {
			e.collectSubnetsPerVpcUsage(allVpcs.Vpcs[i], ec2Svc, *region)
			e.collectInterfaceVpcEndpointsPerVpcUsage(allVpcs.Vpcs[i], ec2Svc, *region)
			e.collectRoutesTablesPerVpcUsage(allVpcs.Vpcs[i], ec2Svc, *region)
			e.collectIPv4BlocksPerVpcUsage(allVpcs.Vpcs[i], ec2Svc, *region)
		}
	}
	e.collectRoutesPerRouteTableQuota(quotaSvc, *region)
	routesCtx, routesCancel := context.WithTimeout(context.Background(), e.timeout)
	defer routesCancel()
	allRouteTables, err := ec2Svc.DescribeRouteTablesWithContext(routesCtx, &ec2.DescribeRouteTablesInput{})
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to DescribeRouteTables failed", "region", region, "err", err)
	} else {
		for i, _ := range allRouteTables.RouteTables {
			e.collectRoutesPerRouteTableUsage(allRouteTables.RouteTables[i], ec2Svc, *region)
		}
	}
}

func (e *VPCExporter) CollectLoop() {
	for {

		wg := &sync.WaitGroup{}
		wg.Add(len(e.sessions))
		for i, _ := range e.sessions {
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

func (e *VPCExporter) GetQuotaValue(client *servicequotas.ServiceQuotas, serviceCode string, quotaCode string) (float64, error) {
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

func (e *VPCExporter) collectVpcsPerRegionQuota(client *servicequotas.ServiceQuotas, region string) {
	quota, err := e.GetQuotaValue(client, SERVICE_CODE_VPC, QUOTA_VPCS_PER_REGION)
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to VpcsPerRegion ServiceQuota failed", "region", region, "err", err)
		exporterMetrics.IncrementErrors()
		return
	}
	e.cache.AddMetric(prometheus.MustNewConstMetric(e.VpcsPerRegionQuota, prometheus.GaugeValue, quota, region))
}

func (e *VPCExporter) collectVpcsPerRegionUsage(ec2Svc *ec2.EC2, region string) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), e.timeout)
	defer cancelFunc()
	describeVpcsOutput, err := ec2Svc.DescribeVpcsWithContext(ctx, &ec2.DescribeVpcsInput{})
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to DescribeVpcs failed", "region", region, "err", err)
		exporterMetrics.IncrementErrors()
		return
	}
	usage := len(describeVpcsOutput.Vpcs)
	e.cache.AddMetric(prometheus.MustNewConstMetric(e.VpcsPerRegionUsage, prometheus.GaugeValue, float64(usage), region))
}

func (e *VPCExporter) collectSubnetsPerVpcQuota(client *servicequotas.ServiceQuotas, region string) {
	quota, err := e.GetQuotaValue(client, SERVICE_CODE_VPC, QUOTA_SUBNETS_PER_VPC)
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to SubnetsPerVpc ServiceQuota failed", "region", region, "err", err)
		exporterMetrics.IncrementErrors()
		return
	}
	e.cache.AddMetric(prometheus.MustNewConstMetric(e.SubnetsPerVpcQuota, prometheus.GaugeValue, quota, region))
}

func (e *VPCExporter) collectSubnetsPerVpcUsage(vpc *ec2.Vpc, ec2Svc *ec2.EC2, region string) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), e.timeout)
	defer cancelFunc()
	describeSubnetsOutput, err := ec2Svc.DescribeSubnetsWithContext(ctx, &ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{&ec2.Filter{
			Name:   aws.String("vpc-id"),
			Values: []*string{vpc.VpcId},
		}},
	})
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to DescribeSubnets failed", "region", region, "err", err)
		exporterMetrics.IncrementErrors()
		return
	}
	usage := len(describeSubnetsOutput.Subnets)
	e.cache.AddMetric(prometheus.MustNewConstMetric(e.SubnetsPerVpcUsage, prometheus.GaugeValue, float64(usage), region, *vpc.VpcId))
}

func (e *VPCExporter) collectRoutesPerRouteTableQuota(client *servicequotas.ServiceQuotas, region string) {
	quota, err := e.GetQuotaValue(client, SERVICE_CODE_VPC, QUOTA_ROUTES_PER_ROUTE_TABLE)
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to RoutesPerRouteTable ServiceQuota failed", "region", region, "err", err)
		exporterMetrics.IncrementErrors()
		return
	}
	e.cache.AddMetric(prometheus.MustNewConstMetric(e.RoutesPerRouteTableQuota, prometheus.GaugeValue, quota, region))
}

func (e *VPCExporter) collectRoutesPerRouteTableUsage(rtb *ec2.RouteTable, ec2Svc *ec2.EC2, region string) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), e.timeout)
	defer cancelFunc()
	descRouteTableOutput, err := ec2Svc.DescribeRouteTablesWithContext(ctx, &ec2.DescribeRouteTablesInput{
		RouteTableIds: []*string{rtb.RouteTableId},
	})
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to DescribeRouteTables failed", "region", region, "err", err)
		exporterMetrics.IncrementErrors()
		return
	}
	quota := len(descRouteTableOutput.RouteTables)
	e.cache.AddMetric(prometheus.MustNewConstMetric(e.RoutesPerRouteTableUsage, prometheus.GaugeValue, float64(quota), region, *rtb.VpcId, *rtb.RouteTableId))
}

func (e *VPCExporter) collectInterfaceVpcEndpointsPerVpcQuota(client *servicequotas.ServiceQuotas, region string) {
	quota, err := e.GetQuotaValue(client, SERVICE_CODE_VPC, QUOTA_INTERFACE_VPC_ENDPOINTS_PER_VPC)
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to InterfaceVpcEndpointsPerVpc ServiceQuota failed", "region", region, "err", err)
		exporterMetrics.IncrementErrors()
		return
	}
	e.cache.AddMetric(prometheus.MustNewConstMetric(e.InterfaceVpcEndpointsPerVpcQuota, prometheus.GaugeValue, quota, region))
}

func (e *VPCExporter) collectInterfaceVpcEndpointsPerVpcUsage(vpc *ec2.Vpc, ec2Svc *ec2.EC2, region string) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), e.timeout)
	defer cancelFunc()
	descVpcEndpoints, err := ec2Svc.DescribeVpcEndpointsWithContext(ctx, &ec2.DescribeVpcEndpointsInput{
		Filters: []*ec2.Filter{{
			Name:   aws.String("vpc-id"),
			Values: []*string{vpc.VpcId},
		}},
	})
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to DescribeVpcEndpoints failed", "region", region, "err", err)
		exporterMetrics.IncrementErrors()
		return
	}
	quota := len(descVpcEndpoints.VpcEndpoints)
	e.cache.AddMetric(prometheus.MustNewConstMetric(e.InterfaceVpcEndpointsPerVpcUsage, prometheus.GaugeValue, float64(quota), region, *vpc.VpcId))
}

func (e *VPCExporter) collectRoutesTablesPerVpcQuota(client *servicequotas.ServiceQuotas, region string) {
	quota, err := e.GetQuotaValue(client, SERVICE_CODE_VPC, QUOTA_ROUTE_TABLES_PER_VPC)
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to RoutesTablesPerVpc ServiceQuota failed", "region", region, "err", err)
		exporterMetrics.IncrementErrors()
		return
	}
	e.cache.AddMetric(prometheus.MustNewConstMetric(e.RouteTablesPerVpcQuota, prometheus.GaugeValue, quota, region))
}

func (e *VPCExporter) collectRoutesTablesPerVpcUsage(vpc *ec2.Vpc, ec2Svc *ec2.EC2, region string) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), e.timeout)
	defer cancelFunc()
	descRouteTables, err := ec2Svc.DescribeRouteTablesWithContext(ctx, &ec2.DescribeRouteTablesInput{
		Filters: []*ec2.Filter{{
			Name:   aws.String("vpc-id"),
			Values: []*string{vpc.VpcId},
		}},
	})
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to DescribeRouteTables failed", "region", region, "err", err)
		exporterMetrics.IncrementErrors()
		return
	}
	quota := len(descRouteTables.RouteTables)
	e.cache.AddMetric(prometheus.MustNewConstMetric(e.RouteTablesPerVpcUsage, prometheus.GaugeValue, float64(quota), region, *vpc.VpcId))
}

func (e *VPCExporter) collectIPv4BlocksPerVpcQuota(client *servicequotas.ServiceQuotas, region string) {
	quota, err := e.GetQuotaValue(client, SERVICE_CODE_VPC, QUOTA_IPV4_BLOCKS_PER_VPC)
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to IPv4BlocksPerVpc ServiceQuota failed", "region", region, "err", err)
		exporterMetrics.IncrementErrors()
		return
	}
	e.cache.AddMetric(prometheus.MustNewConstMetric(e.IPv4BlocksPerVpcQuota, prometheus.GaugeValue, quota, region))
}

func (e *VPCExporter) collectIPv4BlocksPerVpcUsage(vpc *ec2.Vpc, ec2Svc *ec2.EC2, region string) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), e.timeout)
	defer cancelFunc()
	descVpcs, err := ec2Svc.DescribeVpcsWithContext(ctx, &ec2.DescribeVpcsInput{
		VpcIds: []*string{vpc.VpcId},
	})
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to DescribeVpcs failed", "region", region, "err", err)
		exporterMetrics.IncrementErrors()
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
