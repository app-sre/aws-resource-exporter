package main

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

type RDSExporter struct {
	sess             *session.Session
	AllocatedStorage *prometheus.Desc
	DBInstanceClass  *prometheus.Desc
	DBInstanceStatus *prometheus.Desc
	EngineVersion    *prometheus.Desc
}

func NewRDSExporter(sess *session.Session) *RDSExporter {
	return &RDSExporter{
		sess: sess,
		AllocatedStorage: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "allocatedstorage"),
			"The amount of allocated storage in bytes.",
			[]string{"aws_region", "dbinstance_identifier"},
			nil,
		),
		DBInstanceClass: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "dbinstanceclass"),
			"The DB instance class (type).",
			[]string{"aws_region", "dbinstance_identifier", "instance_class"},
			nil,
		),
		DBInstanceStatus: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "dbinstancestatus"),
			"The instance status.",
			[]string{"aws_region", "dbinstance_identifier", "instance_status"},
			nil,
		),
		EngineVersion: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "engineversion"),
			"The DB engine type and version.",
			[]string{"aws_region", "dbinstance_identifier", "engine", "engine_version"},
			nil,
		),
	}
}

func (e *RDSExporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.AllocatedStorage
	ch <- e.DBInstanceStatus
	ch <- e.EngineVersion
}

func (e *RDSExporter) Collect(ch chan<- prometheus.Metric) {
	svc := rds.New(e.sess)
	input := &rds.DescribeDBInstancesInput{}

	// Get all DB instances.
	// If a Marker is found, do pagination until last page
	var instances []*rds.DBInstance
	for {
		exporterMetrics.IncrementRequests()
		result, err := svc.DescribeDBInstances(input)
		if err != nil {
			log.Errorf("[RDS] Call to DescribeDBInstances failed in region %s: %s", *e.sess.Config.Region, err)
			exporterMetrics.IncrementErrors()
			return
		}
		instances = append(instances, result.DBInstances...)
		input.Marker = result.Marker
		if result.Marker == nil {
			break
		}
	}

	parameterGroups := make(map[string][]*rds.Parameter)
	for _, instance := range instances {
		for _, dbpg := range instance.DBParameterGroups {
			if *dbpg.ParameterApplyStatus == "in-sync" {
				if _, ok := parameterGroups[*dbpg.DBParameterGroupName]; ok {
					log.Debugln("ParameterGroup", *dbpg.DBParameterGroupName, "exists in cache.")
					continue
				}

				log.Debugln("Fetching parameters for group", *dbpg.DBParameterGroupName)
				input := &rds.DescribeDBParametersInput{
					DBParameterGroupName: dbpg.DBParameterGroupName,
				}
				var parameters []*rds.Parameter
				for {
					exporterMetrics.IncrementRequests()
					result, err := svc.DescribeDBParameters(input)
					if err != nil {
						log.Errorf("[RDS] Call to DescribeDBInstances failed in region %s: %s", *e.sess.Config.Region, err)
						exporterMetrics.IncrementErrors()
						return
					}
					parameters = append(parameters, result.Parameters...)
					input.Marker = result.Marker
					if result.Marker == nil {
						break
					}
				}
				parameterGroups[*dbpg.DBParameterGroupName] = parameters
			}
		}

		ch <- prometheus.MustNewConstMetric(e.AllocatedStorage, prometheus.GaugeValue, float64(*instance.AllocatedStorage*1024*1024*1024), *e.sess.Config.Region, *instance.DBInstanceIdentifier)
		ch <- prometheus.MustNewConstMetric(e.DBInstanceStatus, prometheus.GaugeValue, 1, *e.sess.Config.Region, *instance.DBInstanceIdentifier, *instance.DBInstanceStatus)
		ch <- prometheus.MustNewConstMetric(e.EngineVersion, prometheus.GaugeValue, 1, *e.sess.Config.Region, *instance.DBInstanceIdentifier, *instance.Engine, *instance.EngineVersion)
		ch <- prometheus.MustNewConstMetric(e.DBInstanceClass, prometheus.GaugeValue, 1, *e.sess.Config.Region, *instance.DBInstanceIdentifier, *instance.DBInstanceClass)
	}
}
