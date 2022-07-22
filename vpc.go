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

func (e *VPCExporter) Collect(ch chan<- prometheus.Metric) {
	wg := &sync.WaitGroup{}
	wg.Add(len(e.sessions))
	for i, _ := range e.sessions {
		go func(index int) {
			defer wg.Done()
			session := e.sessions[index]
			region := session.Config.Region
			ec2Svc := ec2.New(session)
			serviceQuotaSvc := servicequotas.New(session)

			e.collectVpcsPerRegionQuota(region, ec2Svc, serviceQuotaSvc, ch)
			e.collectVpcsPerRegionUsage(region, ec2Svc, ch)
			e.collectRoutesTablesPerVpcQuota(region, ec2Svc, serviceQuotaSvc, ch)
			e.collectInterfaceVpcEndpointsPerVpcQuota(region, ec2Svc, serviceQuotaSvc, ch)
			e.collectSubnetsPerVpcQuota(region, ec2Svc, serviceQuotaSvc, ch)
			e.collectIPv4BlocksPerVpcQuota(region, ec2Svc, serviceQuotaSvc, ch)
			vpcCtx, vpcCancel := context.WithTimeout(context.Background(), e.timeout)
			defer vpcCancel()
			allVpcs, err := ec2Svc.DescribeVpcsWithContext(vpcCtx, &ec2.DescribeVpcsInput{})
			if err != nil {
				level.Error(e.logger).Log("msg", "Call to DescribeVpcs failed", "region", region, "err", err)
			} else {
				for i, _ := range allVpcs.Vpcs {
					e.collectSubnetsPerVpcUsage(region, ec2Svc, ch, allVpcs.Vpcs[i])
					e.collectInterfaceVpcEndpointsPerVpcUsage(region, ec2Svc, ch, allVpcs.Vpcs[i])
					e.collectRoutesTablesPerVpcUsage(region, ec2Svc, ch, allVpcs.Vpcs[i])
					e.collectIPv4BlocksPerVpcUsage(region, ec2Svc, ch, allVpcs.Vpcs[i])
				}
			}
			e.collectRoutesPerRouteTableQuota(region, ec2Svc, serviceQuotaSvc, ch)
			routesCtx, routesCancel := context.WithTimeout(context.Background(), e.timeout)
			defer routesCancel()
			allRouteTables, err := ec2Svc.DescribeRouteTablesWithContext(routesCtx, &ec2.DescribeRouteTablesInput{})
			if err != nil {
				level.Error(e.logger).Log("msg", "Call to DescribeRouteTables failed", "region", region, "err", err)
			} else {
				for i, _ := range allRouteTables.RouteTables {
					e.collectRoutesPerRouteTableUsage(region, ec2Svc, ch, allRouteTables.RouteTables[i])
				}
			}
		}(i)
	}
	wg.Wait()
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

func (e *VPCExporter) collectVpcsPerRegionQuota(region *string, svc *ec2.EC2, serviceQuotasSvc *servicequotas.ServiceQuotas, ch chan<- prometheus.Metric) {
	quota, err := e.GetQuotaValue(serviceQuotasSvc, SERVICE_CODE_VPC, QUOTA_VPCS_PER_REGION)
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to VpcsPerRegion ServiceQuota failed", "region", region, "err", err)
		exporterMetrics.IncrementErrors()
		return
	}
	ch <- prometheus.MustNewConstMetric(e.VpcsPerRegionQuota, prometheus.GaugeValue, quota, *region)
}

func (e *VPCExporter) collectVpcsPerRegionUsage(region *string, ec2Svc *ec2.EC2, ch chan<- prometheus.Metric) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), e.timeout)
	defer cancelFunc()
	describeVpcsOutput, err := ec2Svc.DescribeVpcsWithContext(ctx, &ec2.DescribeVpcsInput{})
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to DescribeVpcs failed", "region", region, "err", err)
		exporterMetrics.IncrementErrors()
		return
	}
	usage := len(describeVpcsOutput.Vpcs)
	ch <- prometheus.MustNewConstMetric(e.VpcsPerRegionUsage, prometheus.GaugeValue, float64(usage), *region)
}

func (e *VPCExporter) collectSubnetsPerVpcQuota(region *string, svc *ec2.EC2, serviceQuotasSvc *servicequotas.ServiceQuotas, ch chan<- prometheus.Metric) {
	quota, err := e.GetQuotaValue(serviceQuotasSvc, SERVICE_CODE_VPC, QUOTA_SUBNETS_PER_VPC)
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to SubnetsPerVpc ServiceQuota failed", "region", region, "err", err)
		exporterMetrics.IncrementErrors()
		return
	}
	ch <- prometheus.MustNewConstMetric(e.SubnetsPerVpcQuota, prometheus.GaugeValue, quota, *region)
}

func (e *VPCExporter) collectSubnetsPerVpcUsage(region *string, svc *ec2.EC2, ch chan<- prometheus.Metric, vpc *ec2.Vpc) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), e.timeout)
	defer cancelFunc()
	describeSubnetsOutput, err := svc.DescribeSubnetsWithContext(ctx, &ec2.DescribeSubnetsInput{
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
	ch <- prometheus.MustNewConstMetric(e.SubnetsPerVpcUsage, prometheus.GaugeValue, float64(usage), *region, *vpc.VpcId)
}

func (e *VPCExporter) collectRoutesPerRouteTableQuota(region *string, svc *ec2.EC2, serviceQuotasSvc *servicequotas.ServiceQuotas, ch chan<- prometheus.Metric) {
	quota, err := e.GetQuotaValue(serviceQuotasSvc, SERVICE_CODE_VPC, QUOTA_ROUTES_PER_ROUTE_TABLE)
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to RoutesPerRouteTable ServiceQuota failed", "region", region, "err", err)
		exporterMetrics.IncrementErrors()
		return
	}
	ch <- prometheus.MustNewConstMetric(e.RoutesPerRouteTableQuota, prometheus.GaugeValue, quota, *region)
}

func (e *VPCExporter) collectRoutesPerRouteTableUsage(region *string, svc *ec2.EC2, ch chan<- prometheus.Metric, rtb *ec2.RouteTable) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), e.timeout)
	defer cancelFunc()
	descRouteTableOutput, err := svc.DescribeRouteTablesWithContext(ctx, &ec2.DescribeRouteTablesInput{
		RouteTableIds: []*string{rtb.RouteTableId},
	})
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to DescribeRouteTables failed", "region", region, "err", err)
		exporterMetrics.IncrementErrors()
		return
	}
	quota := len(descRouteTableOutput.RouteTables)
	ch <- prometheus.MustNewConstMetric(e.RoutesPerRouteTableUsage, prometheus.GaugeValue, float64(quota), *region, *rtb.VpcId, *rtb.RouteTableId)
}

func (e *VPCExporter) collectInterfaceVpcEndpointsPerVpcQuota(region *string, ec2Svc *ec2.EC2, serviceQuotasSvc *servicequotas.ServiceQuotas, ch chan<- prometheus.Metric) {
	quota, err := e.GetQuotaValue(serviceQuotasSvc, SERVICE_CODE_VPC, QUOTA_INTERFACE_VPC_ENDPOINTS_PER_VPC)
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to InterfaceVpcEndpointsPerVpc ServiceQuota failed", "region", region, "err", err)
		exporterMetrics.IncrementErrors()
		return
	}
	ch <- prometheus.MustNewConstMetric(e.InterfaceVpcEndpointsPerVpcQuota, prometheus.GaugeValue, quota, *region)
}

func (e *VPCExporter) collectInterfaceVpcEndpointsPerVpcUsage(region *string, ec2Svc *ec2.EC2, ch chan<- prometheus.Metric, vpc *ec2.Vpc) {
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
	ch <- prometheus.MustNewConstMetric(e.InterfaceVpcEndpointsPerVpcUsage, prometheus.GaugeValue, float64(quota), *region, *vpc.VpcId)
}

func (e *VPCExporter) collectRoutesTablesPerVpcQuota(region *string, ec2Svc *ec2.EC2, serviceQuotasSvc *servicequotas.ServiceQuotas, ch chan<- prometheus.Metric) {
	quota, err := e.GetQuotaValue(serviceQuotasSvc, SERVICE_CODE_VPC, QUOTA_ROUTE_TABLES_PER_VPC)
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to RoutesTablesPerVpc ServiceQuota failed", "region", region, "err", err)
		exporterMetrics.IncrementErrors()
		return
	}
	ch <- prometheus.MustNewConstMetric(e.RouteTablesPerVpcQuota, prometheus.GaugeValue, quota, *region)
}

func (e *VPCExporter) collectRoutesTablesPerVpcUsage(region *string, ec2Svc *ec2.EC2, ch chan<- prometheus.Metric, vpc *ec2.Vpc) {
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
	ch <- prometheus.MustNewConstMetric(e.RouteTablesPerVpcUsage, prometheus.GaugeValue, float64(quota), *region, *vpc.VpcId)
}

func (e *VPCExporter) collectIPv4BlocksPerVpcQuota(region *string, ec2Svc *ec2.EC2, serviceQuotasSvc *servicequotas.ServiceQuotas, ch chan<- prometheus.Metric) {
	quota, err := e.GetQuotaValue(serviceQuotasSvc, SERVICE_CODE_VPC, QUOTA_IPV4_BLOCKS_PER_VPC)
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to IPv4BlocksPerVpc ServiceQuota failed", "region", region, "err", err)
		exporterMetrics.IncrementErrors()
		return
	}
	ch <- prometheus.MustNewConstMetric(e.IPv4BlocksPerVpcQuota, prometheus.GaugeValue, quota, *region)
}

func (e *VPCExporter) collectIPv4BlocksPerVpcUsage(region *string, ec2Svc *ec2.EC2, ch chan<- prometheus.Metric, vpc *ec2.Vpc) {
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
	ch <- prometheus.MustNewConstMetric(e.IPv4BlocksPerVpcUsage, prometheus.GaugeValue, float64(quota), *region, *vpc.VpcId)
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
