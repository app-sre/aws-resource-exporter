package main

import (
	"context"
	"sync"
	"time"

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

	logger  log.Logger
	timeout time.Duration
}

type VPCCollector struct {
	e             *VPCExporter
	ec2           *ec2.EC2
	serviceQuotas *servicequotas.ServiceQuotas
	region        *string
	wg            *sync.WaitGroup
}

func NewVPCExporter(sess []*session.Session, logger log.Logger, timeout time.Duration) *VPCExporter {
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
		timeout:                          timeout,
	}
}

func (c *VPCCollector) Collect(ch chan<- prometheus.Metric) {
	defer c.wg.Done()

	c.collectVpcsPerRegionQuota(ch)
	c.collectVpcsPerRegionUsage(ch)
	c.collectRoutesTablesPerVpcQuota(ch)
	c.collectInterfaceVpcEndpointsPerVpcQuota(ch)
	c.collectSubnetsPerVpcQuota(ch)
	c.collectIPv4BlocksPerVpcQuota(ch)
	vpcCtx, vpcCancel := context.WithTimeout(context.Background(), c.e.timeout)
	defer vpcCancel()
	allVpcs, err := c.ec2.DescribeVpcsWithContext(vpcCtx, &ec2.DescribeVpcsInput{})
	if err != nil {
		level.Error(c.e.logger).Log("msg", "Call to DescribeVpcs failed", "region", c.region, "err", err)
	} else {
		for i, _ := range allVpcs.Vpcs {
			c.collectSubnetsPerVpcUsage(ch, allVpcs.Vpcs[i])
			c.collectInterfaceVpcEndpointsPerVpcUsage(ch, allVpcs.Vpcs[i])
			c.collectRoutesTablesPerVpcUsage(ch, allVpcs.Vpcs[i])
			c.collectIPv4BlocksPerVpcUsage(ch, allVpcs.Vpcs[i])
		}
	}
	c.collectRoutesPerRouteTableQuota(ch)
	routesCtx, routesCancel := context.WithTimeout(context.Background(), c.e.timeout)
	defer routesCancel()
	allRouteTables, err := c.ec2.DescribeRouteTablesWithContext(routesCtx, &ec2.DescribeRouteTablesInput{})
	if err != nil {
		level.Error(c.e.logger).Log("msg", "Call to DescribeRouteTables failed", "region", c.region, "err", err)
	} else {
		for i, _ := range allRouteTables.RouteTables {
			c.collectRoutesPerRouteTableUsage(ch, allRouteTables.RouteTables[i])
		}
	}
}

func (e *VPCExporter) Collect(ch chan<- prometheus.Metric) {
	wg := &sync.WaitGroup{}
	wg.Add(len(e.sessions))
	for i, _ := range e.sessions {
		session := e.sessions[i]
		region := session.Config.Region
		collector := VPCCollector{
			e:             e,
			ec2:           ec2.New(session),
			serviceQuotas: servicequotas.New(session),
			region:        region,
			wg:            wg,
		}
		go collector.Collect(ch)
	}
	wg.Wait()
}

func (c *VPCCollector) GetQuotaValue(client *servicequotas.ServiceQuotas, serviceCode string, quotaCode string) (float64, error) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), c.e.timeout)
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

func (c *VPCCollector) collectVpcsPerRegionQuota(ch chan<- prometheus.Metric) {
	quota, err := c.GetQuotaValue(c.serviceQuotas, SERVICE_CODE_VPC, QUOTA_VPCS_PER_REGION)
	if err != nil {
		level.Error(c.e.logger).Log("msg", "Call to VpcsPerRegion ServiceQuota failed", "region", c.region, "err", err)
		exporterMetrics.IncrementErrors()
		return
	}
	ch <- prometheus.MustNewConstMetric(c.e.VpcsPerRegionQuota, prometheus.GaugeValue, quota, *c.region)
}

func (c *VPCCollector) collectVpcsPerRegionUsage(ch chan<- prometheus.Metric) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), c.e.timeout)
	defer cancelFunc()
	describeVpcsOutput, err := c.ec2.DescribeVpcsWithContext(ctx, &ec2.DescribeVpcsInput{})
	if err != nil {
		level.Error(c.e.logger).Log("msg", "Call to DescribeVpcs failed", "region", c.region, "err", err)
		exporterMetrics.IncrementErrors()
		return
	}
	usage := len(describeVpcsOutput.Vpcs)
	ch <- prometheus.MustNewConstMetric(c.e.VpcsPerRegionUsage, prometheus.GaugeValue, float64(usage), *c.region)
}

func (c *VPCCollector) collectSubnetsPerVpcQuota(ch chan<- prometheus.Metric) {
	quota, err := c.GetQuotaValue(c.serviceQuotas, SERVICE_CODE_VPC, QUOTA_SUBNETS_PER_VPC)
	if err != nil {
		level.Error(c.e.logger).Log("msg", "Call to SubnetsPerVpc ServiceQuota failed", "region", c.region, "err", err)
		exporterMetrics.IncrementErrors()
		return
	}
	ch <- prometheus.MustNewConstMetric(c.e.SubnetsPerVpcQuota, prometheus.GaugeValue, quota, *c.region)
}

func (c *VPCCollector) collectSubnetsPerVpcUsage(ch chan<- prometheus.Metric, vpc *ec2.Vpc) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), c.e.timeout)
	defer cancelFunc()
	describeSubnetsOutput, err := c.ec2.DescribeSubnetsWithContext(ctx, &ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{&ec2.Filter{
			Name:   aws.String("vpc-id"),
			Values: []*string{vpc.VpcId},
		}},
	})
	if err != nil {
		level.Error(c.e.logger).Log("msg", "Call to DescribeSubnets failed", "region", c.region, "err", err)
		exporterMetrics.IncrementErrors()
		return
	}
	usage := len(describeSubnetsOutput.Subnets)
	ch <- prometheus.MustNewConstMetric(c.e.SubnetsPerVpcUsage, prometheus.GaugeValue, float64(usage), *c.region, *vpc.VpcId)
}

func (c *VPCCollector) collectRoutesPerRouteTableQuota(ch chan<- prometheus.Metric) {
	quota, err := c.GetQuotaValue(c.serviceQuotas, SERVICE_CODE_VPC, QUOTA_ROUTES_PER_ROUTE_TABLE)
	if err != nil {
		level.Error(c.e.logger).Log("msg", "Call to RoutesPerRouteTable ServiceQuota failed", "region", c.region, "err", err)
		exporterMetrics.IncrementErrors()
		return
	}
	ch <- prometheus.MustNewConstMetric(c.e.RoutesPerRouteTableQuota, prometheus.GaugeValue, quota, *c.region)
}

func (c *VPCCollector) collectRoutesPerRouteTableUsage(ch chan<- prometheus.Metric, rtb *ec2.RouteTable) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), c.e.timeout)
	defer cancelFunc()
	descRouteTableOutput, err := c.ec2.DescribeRouteTablesWithContext(ctx, &ec2.DescribeRouteTablesInput{
		RouteTableIds: []*string{rtb.RouteTableId},
	})
	if err != nil {
		level.Error(c.e.logger).Log("msg", "Call to DescribeRouteTables failed", "region", c.region, "err", err)
		exporterMetrics.IncrementErrors()
		return
	}
	quota := len(descRouteTableOutput.RouteTables)
	ch <- prometheus.MustNewConstMetric(c.e.RoutesPerRouteTableUsage, prometheus.GaugeValue, float64(quota), *c.region, *rtb.VpcId, *rtb.RouteTableId)
}

func (c *VPCCollector) collectInterfaceVpcEndpointsPerVpcQuota(ch chan<- prometheus.Metric) {
	quota, err := c.GetQuotaValue(c.serviceQuotas, SERVICE_CODE_VPC, QUOTA_INTERFACE_VPC_ENDPOINTS_PER_VPC)
	if err != nil {
		level.Error(c.e.logger).Log("msg", "Call to InterfaceVpcEndpointsPerVpc ServiceQuota failed", "region", c.region, "err", err)
		exporterMetrics.IncrementErrors()
		return
	}
	ch <- prometheus.MustNewConstMetric(c.e.InterfaceVpcEndpointsPerVpcQuota, prometheus.GaugeValue, quota, *c.region)
}

func (c *VPCCollector) collectInterfaceVpcEndpointsPerVpcUsage(ch chan<- prometheus.Metric, vpc *ec2.Vpc) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), c.e.timeout)
	defer cancelFunc()
	descVpcEndpoints, err := c.ec2.DescribeVpcEndpointsWithContext(ctx, &ec2.DescribeVpcEndpointsInput{
		Filters: []*ec2.Filter{{
			Name:   aws.String("vpc-id"),
			Values: []*string{vpc.VpcId},
		}},
	})
	if err != nil {
		level.Error(c.e.logger).Log("msg", "Call to DescribeVpcEndpoints failed", "region", c.region, "err", err)
		exporterMetrics.IncrementErrors()
		return
	}
	quota := len(descVpcEndpoints.VpcEndpoints)
	ch <- prometheus.MustNewConstMetric(c.e.InterfaceVpcEndpointsPerVpcUsage, prometheus.GaugeValue, float64(quota), *c.region, *vpc.VpcId)
}

func (c *VPCCollector) collectRoutesTablesPerVpcQuota(ch chan<- prometheus.Metric) {
	quota, err := c.GetQuotaValue(c.serviceQuotas, SERVICE_CODE_VPC, QUOTA_ROUTE_TABLES_PER_VPC)
	if err != nil {
		level.Error(c.e.logger).Log("msg", "Call to RoutesTablesPerVpc ServiceQuota failed", "region", c.region, "err", err)
		exporterMetrics.IncrementErrors()
		return
	}
	ch <- prometheus.MustNewConstMetric(c.e.RouteTablesPerVpcQuota, prometheus.GaugeValue, quota, *c.region)
}

func (c *VPCCollector) collectRoutesTablesPerVpcUsage(ch chan<- prometheus.Metric, vpc *ec2.Vpc) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), c.e.timeout)
	defer cancelFunc()
	descRouteTables, err := c.ec2.DescribeRouteTablesWithContext(ctx, &ec2.DescribeRouteTablesInput{
		Filters: []*ec2.Filter{{
			Name:   aws.String("vpc-id"),
			Values: []*string{vpc.VpcId},
		}},
	})
	if err != nil {
		level.Error(c.e.logger).Log("msg", "Call to DescribeRouteTables failed", "region", c.region, "err", err)
		exporterMetrics.IncrementErrors()
		return
	}
	quota := len(descRouteTables.RouteTables)
	ch <- prometheus.MustNewConstMetric(c.e.RouteTablesPerVpcUsage, prometheus.GaugeValue, float64(quota), *c.region, *vpc.VpcId)
}

func (c *VPCCollector) collectIPv4BlocksPerVpcQuota(ch chan<- prometheus.Metric) {
	quota, err := c.GetQuotaValue(c.serviceQuotas, SERVICE_CODE_VPC, QUOTA_IPV4_BLOCKS_PER_VPC)
	if err != nil {
		level.Error(c.e.logger).Log("msg", "Call to IPv4BlocksPerVpc ServiceQuota failed", "region", c.region, "err", err)
		exporterMetrics.IncrementErrors()
		return
	}
	ch <- prometheus.MustNewConstMetric(c.e.IPv4BlocksPerVpcQuota, prometheus.GaugeValue, quota, *c.region)
}

func (c *VPCCollector) collectIPv4BlocksPerVpcUsage(ch chan<- prometheus.Metric, vpc *ec2.Vpc) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), c.e.timeout)
	defer cancelFunc()
	descVpcs, err := c.ec2.DescribeVpcsWithContext(ctx, &ec2.DescribeVpcsInput{
		VpcIds: []*string{vpc.VpcId},
	})
	if err != nil {
		level.Error(c.e.logger).Log("msg", "Call to DescribeVpcs failed", "region", c.region, "err", err)
		exporterMetrics.IncrementErrors()
		return
	}
	if len(descVpcs.Vpcs) != 1 {
		level.Error(c.e.logger).Log("msg", "Unexpected numbers of VPCs (!= 1) returned", "region", c.region, "vpcId", vpc.VpcId)
	}
	quota := len(descVpcs.Vpcs[0].CidrBlockAssociationSet)
	ch <- prometheus.MustNewConstMetric(c.e.IPv4BlocksPerVpcUsage, prometheus.GaugeValue, float64(quota), *c.region, *vpc.VpcId)
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
