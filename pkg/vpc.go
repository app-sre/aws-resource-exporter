package pkg

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/app-sre/aws-resource-exporter/pkg/awsclient"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2_types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/servicequotas"
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
	AWS_RESERVED_IPS_PER_SUBNET           int64  = 5
)

type VPCExporter struct {
	awsAccountId                     string
	configs                          []aws.Config
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
	IPv4AddressesPerSubnetQuota      *prometheus.Desc
	IPv4AddressesPerSubnetUsage      *prometheus.Desc

	logger   *slog.Logger
	timeout  time.Duration
	cache    MetricsCache
	interval time.Duration
}

type VPCCollector struct {
	e             *VPCExporter
	ec2           *ec2.Client
	serviceQuotas *servicequotas.Client
	region        *string
	wg            *sync.WaitGroup
}

func NewVPCExporter(configs []aws.Config, logger *slog.Logger, config VPCConfig, awsAccountId string) *VPCExporter {
	logger.Info("Initializing VPC exporter")
	constLabels := map[string]string{"aws_account_id": awsAccountId, SERVICE_CODE_KEY: SERVICE_CODE_VPC}
	return &VPCExporter{
		awsAccountId:                     awsAccountId,
		configs:                          configs,
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
		IPv4AddressesPerSubnetQuota:      prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "vpc_ipv4addressespersubnet_quota"), "The quota of IPv4 addresses per subnet (based on CIDR)", []string{"aws_region", "vpcid", "subnetid"}, constLabels),
		IPv4AddressesPerSubnetUsage:      prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "vpc_ipv4addressespersubnet_usage"), "The usage of IPv4 addresses per subnet", []string{"aws_region", "vpcid", "subnetid"}, constLabels),
		logger:                           logger,
		timeout:                          *config.Timeout,
		cache:                            *NewMetricsCache(*config.CacheTTL),
		interval:                         *config.Interval,
	}
}

func (e *VPCExporter) CollectInRegion(cfg aws.Config, region string, wg *sync.WaitGroup) {
	defer wg.Done()

	ec2Svc := ec2.NewFromConfig(cfg)
	quotaSvc := servicequotas.NewFromConfig(cfg)

	e.collectVpcsPerRegionQuota(quotaSvc, region)
	e.collectVpcsPerRegionUsage(ec2Svc, region)
	e.collectRoutesTablesPerVpcQuota(quotaSvc, region)
	e.collectInterfaceVpcEndpointsPerVpcQuota(quotaSvc, region)
	e.collectSubnetsPerVpcQuota(quotaSvc, region)
	e.collectIPv4BlocksPerVpcQuota(quotaSvc, region)
	vpcCtx, vpcCancel := context.WithTimeout(context.Background(), e.timeout)
	defer vpcCancel()
	allVpcs, err := ec2Svc.DescribeVpcs(vpcCtx, &ec2.DescribeVpcsInput{})
	if err != nil {
		e.logger.Error("Call to DescribeVpcs failed", "region", region, "err", err)
	} else {
		for i, _ := range allVpcs.Vpcs {
			e.collectSubnetsPerVpcUsage(allVpcs.Vpcs[i], ec2Svc, region)
			e.collectInterfaceVpcEndpointsPerVpcUsage(allVpcs.Vpcs[i], ec2Svc, region)
			e.collectRoutesTablesPerVpcUsage(allVpcs.Vpcs[i], ec2Svc, region)
			e.collectIPv4BlocksPerVpcUsage(allVpcs.Vpcs[i], ec2Svc, region)
			e.collectIPv4AddressesPerSubnetUsage(allVpcs.Vpcs[i], ec2Svc, region)
		}
	}
	e.collectRoutesPerRouteTableQuota(quotaSvc, region)
	routesCtx, routesCancel := context.WithTimeout(context.Background(), e.timeout)
	defer routesCancel()
	allRouteTables, err := ec2Svc.DescribeRouteTables(routesCtx, &ec2.DescribeRouteTablesInput{})
	if err != nil {
		e.logger.Error("Call to DescribeRouteTables failed", "region", region, "err", err)
	} else {
		for i, _ := range allRouteTables.RouteTables {
			e.collectRoutesPerRouteTableUsage(allRouteTables.RouteTables[i], ec2Svc, region)
		}
	}
}

func (e *VPCExporter) CollectLoop() {
	for {

		wg := &sync.WaitGroup{}
		wg.Add(len(e.configs))
		for i, _ := range e.configs {
			cfg := e.configs[i]
			region := cfg.Region
			go e.CollectInRegion(cfg, region, wg)
		}
		wg.Wait()

		e.logger.Info("VPC metrics Updated")

		time.Sleep(e.interval)
	}
}

func (e *VPCExporter) Collect(ch chan<- prometheus.Metric) {
	for _, m := range e.cache.GetAllMetrics() {
		ch <- m
	}
}

func (e *VPCExporter) GetQuotaValue(client *servicequotas.Client, serviceCode string, quotaCode string) (float64, error) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), e.timeout)
	defer cancelFunc()
	sqOutput, err := client.GetServiceQuota(ctx, &servicequotas.GetServiceQuotaInput{
		QuotaCode:   aws.String(quotaCode),
		ServiceCode: aws.String(serviceCode),
	})

	if err != nil {
		return 0, err
	}
	// It seems sometimes the returned Quota contains a nil value - probably because the Value is "Required: No"
	// https://docs.aws.amazon.com/servicequotas/2019-06-24/apireference/API_ServiceQuota.html#servicequotas-Type-ServiceQuota-Value
	if sqOutput.Quota == nil || sqOutput.Quota.Value == nil {
		e.logger.Error("VPC Quota was nil", "quota-code", quotaCode)
		return 0, errors.New("VPC Quota was nil")
	}
	return *sqOutput.Quota.Value, nil
}

func (e *VPCExporter) collectVpcsPerRegionQuota(client *servicequotas.Client, region string) {
	quota, err := e.GetQuotaValue(client, SERVICE_CODE_VPC, QUOTA_VPCS_PER_REGION)
	if err != nil {
		e.logger.Error("Call to VpcsPerRegion ServiceQuota failed", "region", region, "err", err)
		awsclient.AwsExporterMetrics.IncrementErrors()
		return
	}
	e.cache.AddMetric(prometheus.MustNewConstMetric(e.VpcsPerRegionQuota, prometheus.GaugeValue, quota, region))
}

func (e *VPCExporter) collectVpcsPerRegionUsage(ec2Svc *ec2.Client, region string) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), e.timeout)
	defer cancelFunc()
	describeVpcsOutput, err := ec2Svc.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{})
	if err != nil {
		e.logger.Error("Call to DescribeVpcs failed", "region", region, "err", err)
		awsclient.AwsExporterMetrics.IncrementErrors()
		return
	}
	usage := len(describeVpcsOutput.Vpcs)
	e.cache.AddMetric(prometheus.MustNewConstMetric(e.VpcsPerRegionUsage, prometheus.GaugeValue, float64(usage), region))
}

func (e *VPCExporter) collectSubnetsPerVpcQuota(client *servicequotas.Client, region string) {
	quota, err := e.GetQuotaValue(client, SERVICE_CODE_VPC, QUOTA_SUBNETS_PER_VPC)
	if err != nil {
		e.logger.Error("Call to SubnetsPerVpc ServiceQuota failed", "region", region, "err", err)
		awsclient.AwsExporterMetrics.IncrementErrors()
		return
	}
	e.cache.AddMetric(prometheus.MustNewConstMetric(e.SubnetsPerVpcQuota, prometheus.GaugeValue, quota, region))
}

func (e *VPCExporter) collectSubnetsPerVpcUsage(vpc ec2_types.Vpc, ec2Svc *ec2.Client, region string) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), e.timeout)
	defer cancelFunc()
	describeSubnetsOutput, err := ec2Svc.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
		Filters: []ec2_types.Filter{{
			Name:   aws.String("vpc-id"),
			Values: []string{*vpc.VpcId},
		}},
	})
	if err != nil {
		e.logger.Error("Call to DescribeSubnets failed", "region", region, "err", err)
		awsclient.AwsExporterMetrics.IncrementErrors()
		return
	}
	usage := len(describeSubnetsOutput.Subnets)
	e.cache.AddMetric(prometheus.MustNewConstMetric(e.SubnetsPerVpcUsage, prometheus.GaugeValue, float64(usage), region, *vpc.VpcId))
}

func (e *VPCExporter) collectRoutesPerRouteTableQuota(client *servicequotas.Client, region string) {
	quota, err := e.GetQuotaValue(client, SERVICE_CODE_VPC, QUOTA_ROUTES_PER_ROUTE_TABLE)
	if err != nil {
		e.logger.Error("Call to RoutesPerRouteTable ServiceQuota failed", "region", region, "err", err)
		awsclient.AwsExporterMetrics.IncrementErrors()
		return
	}
	e.cache.AddMetric(prometheus.MustNewConstMetric(e.RoutesPerRouteTableQuota, prometheus.GaugeValue, quota, region))
}

func (e *VPCExporter) collectRoutesPerRouteTableUsage(rtb ec2_types.RouteTable, ec2Svc *ec2.Client, region string) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), e.timeout)
	defer cancelFunc()
	descRouteTableOutput, err := ec2Svc.DescribeRouteTables(ctx, &ec2.DescribeRouteTablesInput{
		RouteTableIds: []string{*rtb.RouteTableId},
	})
	if err != nil {
		e.logger.Error("Call to DescribeRouteTables failed", "region", region, "err", err)
		awsclient.AwsExporterMetrics.IncrementErrors()
		return
	}
	quota := len(descRouteTableOutput.RouteTables)
	e.cache.AddMetric(prometheus.MustNewConstMetric(e.RoutesPerRouteTableUsage, prometheus.GaugeValue, float64(quota), region, *rtb.VpcId, *rtb.RouteTableId))
}

func (e *VPCExporter) collectInterfaceVpcEndpointsPerVpcQuota(client *servicequotas.Client, region string) {
	quota, err := e.GetQuotaValue(client, SERVICE_CODE_VPC, QUOTA_INTERFACE_VPC_ENDPOINTS_PER_VPC)
	if err != nil {
		e.logger.Error("Call to InterfaceVpcEndpointsPerVpc ServiceQuota failed", "region", region, "err", err)
		awsclient.AwsExporterMetrics.IncrementErrors()
		return
	}
	e.cache.AddMetric(prometheus.MustNewConstMetric(e.InterfaceVpcEndpointsPerVpcQuota, prometheus.GaugeValue, quota, region))
}

func (e *VPCExporter) collectInterfaceVpcEndpointsPerVpcUsage(vpc ec2_types.Vpc, ec2Svc *ec2.Client, region string) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), e.timeout)
	defer cancelFunc()
	descVpcEndpoints, err := ec2Svc.DescribeVpcEndpoints(ctx, &ec2.DescribeVpcEndpointsInput{
		Filters: []ec2_types.Filter{{
			Name:   aws.String("vpc-id"),
			Values: []string{*vpc.VpcId},
		}},
	})
	if err != nil {
		e.logger.Error("Call to DescribeVpcEndpoints failed", "region", region, "err", err)
		awsclient.AwsExporterMetrics.IncrementErrors()
		return
	}
	quota := len(descVpcEndpoints.VpcEndpoints)
	e.cache.AddMetric(prometheus.MustNewConstMetric(e.InterfaceVpcEndpointsPerVpcUsage, prometheus.GaugeValue, float64(quota), region, *vpc.VpcId))
}

func (e *VPCExporter) collectRoutesTablesPerVpcQuota(client *servicequotas.Client, region string) {
	quota, err := e.GetQuotaValue(client, SERVICE_CODE_VPC, QUOTA_ROUTE_TABLES_PER_VPC)
	if err != nil {
		e.logger.Error("Call to RoutesTablesPerVpc ServiceQuota failed", "region", region, "err", err)
		awsclient.AwsExporterMetrics.IncrementErrors()
		return
	}
	e.cache.AddMetric(prometheus.MustNewConstMetric(e.RouteTablesPerVpcQuota, prometheus.GaugeValue, quota, region))
}

func (e *VPCExporter) collectRoutesTablesPerVpcUsage(vpc ec2_types.Vpc, ec2Svc *ec2.Client, region string) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), e.timeout)
	defer cancelFunc()
	descRouteTables, err := ec2Svc.DescribeRouteTables(ctx, &ec2.DescribeRouteTablesInput{
		Filters: []ec2_types.Filter{{
			Name:   aws.String("vpc-id"),
			Values: []string{*vpc.VpcId},
		}},
	})
	if err != nil {
		e.logger.Error("Call to DescribeRouteTables failed", "region", region, "err", err)
		awsclient.AwsExporterMetrics.IncrementErrors()
		return
	}
	quota := len(descRouteTables.RouteTables)
	e.cache.AddMetric(prometheus.MustNewConstMetric(e.RouteTablesPerVpcUsage, prometheus.GaugeValue, float64(quota), region, *vpc.VpcId))
}

func (e *VPCExporter) collectIPv4BlocksPerVpcQuota(client *servicequotas.Client, region string) {
	quota, err := e.GetQuotaValue(client, SERVICE_CODE_VPC, QUOTA_IPV4_BLOCKS_PER_VPC)
	if err != nil {
		e.logger.Error("Call to IPv4BlocksPerVpc ServiceQuota failed", "region", region, "err", err)
		awsclient.AwsExporterMetrics.IncrementErrors()
		return
	}
	e.cache.AddMetric(prometheus.MustNewConstMetric(e.IPv4BlocksPerVpcQuota, prometheus.GaugeValue, quota, region))
}

func (e *VPCExporter) collectIPv4BlocksPerVpcUsage(vpc ec2_types.Vpc, ec2Svc *ec2.Client, region string) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), e.timeout)
	defer cancelFunc()
	descVpcs, err := ec2Svc.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
		VpcIds: []string{*vpc.VpcId},
	})
	if err != nil {
		e.logger.Error("Call to DescribeVpcs failed", "region", region, "err", err)
		awsclient.AwsExporterMetrics.IncrementErrors()
		return
	}
	if len(descVpcs.Vpcs) != 1 {
		e.logger.Error("Unexpected numbers of VPCs (!= 1) returned", "region", region, "vpcId", *vpc.VpcId)
	}
	quota := len(descVpcs.Vpcs[0].CidrBlockAssociationSet)
	e.cache.AddMetric(prometheus.MustNewConstMetric(e.IPv4BlocksPerVpcUsage, prometheus.GaugeValue, float64(quota), region, *vpc.VpcId))
}

func (e *VPCExporter) collectIPv4AddressesPerSubnetUsage(vpc ec2_types.Vpc, ec2Svc *ec2.Client, region string) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), e.timeout)
	defer cancelFunc()
	describeSubnetsOutput, err := ec2Svc.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
		Filters: []ec2_types.Filter{{
			Name:   aws.String("vpc-id"),
			Values: []string{*vpc.VpcId},
		}},
	})
	if err != nil {
		e.logger.Error("Call to DescribeSubnets failed", "region", region, "err", err)
		awsclient.AwsExporterMetrics.IncrementErrors()
		return
	}

	for _, subnet := range describeSubnetsOutput.Subnets {
		// Validate required fields
		if subnet.SubnetId == nil {
			e.logger.Error("Subnet has nil SubnetId", "region", region, "vpcId", *vpc.VpcId)
			awsclient.AwsExporterMetrics.IncrementErrors()
			continue
		}
		if subnet.CidrBlock == nil {
			e.logger.Error("Subnet has nil CidrBlock", "region", region, "subnetId", *subnet.SubnetId)
			awsclient.AwsExporterMetrics.IncrementErrors()
			continue
		}
		if subnet.AvailableIpAddressCount == nil {
			e.logger.Error("Subnet has nil AvailableIpAddressCount", "region", region, "subnetId", *subnet.SubnetId)
			awsclient.AwsExporterMetrics.IncrementErrors()
			continue
		}

		// Calculate total IPs from CIDR block
		cidrBlock := *subnet.CidrBlock
		totalIPs, err := CalculateTotalIPsFromCIDR(cidrBlock, e.logger)
		if err != nil {
			e.logger.Error("Could not calculate total IPs from CIDR", "region", region, "subnetId", *subnet.SubnetId, "cidr", cidrBlock, "err", err)
			awsclient.AwsExporterMetrics.IncrementErrors()
			continue
		}

		// AWS reserves 5 IPs per subnet, so usable IPs = total - 5
		// https://docs.aws.amazon.com/vpc/latest/userguide/subnet-sizing.html
		usableIPs := totalIPs - AWS_RESERVED_IPS_PER_SUBNET
		availableIPs := int64(*subnet.AvailableIpAddressCount)
		usedIPs := usableIPs - availableIPs

		// Validate that used IPs is not negative (sanity check)
		if usedIPs < 0 {
			e.logger.Error("Calculated negative used IPs", "region", region, "subnetId", *subnet.SubnetId, "usableIPs", usableIPs, "availableIPs", availableIPs)
			awsclient.AwsExporterMetrics.IncrementErrors()
			continue
		}

		// Add both quota and usage metrics
		e.cache.AddMetric(prometheus.MustNewConstMetric(e.IPv4AddressesPerSubnetQuota, prometheus.GaugeValue, float64(usableIPs), region, *vpc.VpcId, *subnet.SubnetId))
		e.cache.AddMetric(prometheus.MustNewConstMetric(e.IPv4AddressesPerSubnetUsage, prometheus.GaugeValue, float64(usedIPs), region, *vpc.VpcId, *subnet.SubnetId))
	}
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
	ch <- e.IPv4AddressesPerSubnetQuota
	ch <- e.IPv4AddressesPerSubnetUsage
}
