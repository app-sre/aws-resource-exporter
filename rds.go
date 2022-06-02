package main

import (
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

// DBMaxConnections is a hardcoded map of instance types and DB Parameter Group names
// This is a dump workaround created because by default the DB Parameter Group `max_connections` is a function
// that is hard to parse and process in code and it contains a variable whose value is unknown to us (DBInstanceClassMemory)
// AWS has no means to return the actual `max_connections` value.
// For Aurora see: https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/AuroraMySQL.Managing.Performance.html
var DBMaxConnections = map[string]map[string]int64{
	"db.t2.micro": map[string]int64{
		"default": 87,
	},
	"db.t2.small": map[string]int64{
		"default":          150,
		"default.mysql5.7": 150,
	},
	"db.t3.micro": map[string]int64{
		"default":            112,
		"default.postgres10": 112,
		"default.postgres11": 112,
		"default.postgres12": 112,
		"default.postgres13": 112,
		"default.postgres14": 112,
	},
	"db.t3.small": map[string]int64{
		"default":            225,
		"default.postgres10": 225,
		"default.postgres11": 225,
		"default.postgres12": 225,
		"default.postgres13": 225,
		"default.postgres14": 225,
	},
	"db.t3.medium": map[string]int64{
		"default":            550,
		"default.postgres10": 550,
		"default.postgres11": 550,
		"default.postgres12": 550,
		"default.postgres13": 550,
		"default.postgres14": 550,
	},
	"db.m3.medium": map[string]int64{
		"default": 392,
	},
	"db.m3.large": map[string]int64{
		"default": 801,
	},
	"db.m3.2xlarge": map[string]int64{
		"default": 3379,
	},
	"db.m4.large": map[string]int64{
		"default":            823,
		"default.postgres10": 823,
		"default.postgres11": 823,
		"default.postgres12": 823,
		"default.postgres13": 823,
		"default.postgres14": 823,
	},
	"db.m5.large": map[string]int64{
		"default":            823,
		"default.postgres10": 823,
		"default.postgres11": 823,
		"default.postgres12": 823,
		"default.postgres13": 823,
		"default.postgres14": 823,
	},
	"db.m5.xlarge": map[string]int64{
		"default":            1646,
		"default.postgres10": 1646,
		"default.postgres11": 1646,
		"default.postgres12": 1646,
		"default.postgres13": 1646,
		"default.postgres14": 1646,
	},
	"db.m5.2xlarge": map[string]int64{
		"default":            3429,
		"default.postgres10": 3429,
		"default.postgres11": 3429,
		"default.postgres12": 3429,
		"default.postgres13": 3429,
		"default.postgres14": 3429,
	},
	"db.m5.4xlarge": map[string]int64{
		"default":            5000,
		"default.postgres10": 5000,
		"default.postgres11": 5000,
		"default.postgres12": 5000,
		"default.postgres13": 5000,
		"default.postgres14": 5000,
	},
	"db.r4.large": map[string]int64{
		"default":          1301,
		"default.mysql5.7": 1301,
	},
	"db.r4.4xlarge": map[string]int64{
		"default":          10410,
		"default.mysql5.7": 10410,
	},
	"db.r5.large": map[string]int64{
		"default":            1802,
		"default.postgres10": 1802,
		"default.postgres11": 1802,
		"default.postgres12": 1802,
		"default.postgres13": 1802,
		"default.postgres14": 1802,
	},
	"db.r5.xlarge": map[string]int64{
		"default":            2730,
		"default.mysql5.7":   2730,
		"default.postgres10": 3604,
		"default.postgres11": 3604,
		"default.postgres12": 3604,
		"default.postgres13": 3604,
		"default.postgres14": 3604,
	},
	"db.r5.2xlarge": map[string]int64{
		"default":                 3000,
		"default.aurora-mysql5.7": 3000,
	},
	"db.r5.4xlarge": map[string]int64{
		"default":            5000,
		"default.postgres10": 5000,
		"default.postgres11": 5000,
		"default.postgres12": 5000,
		"default.postgres13": 5000,
		"default.postgres14": 5000,
	},
	"db.r5.8xlarge": map[string]int64{
		"default":          21845,
		"default.mysql5.7": 21845,
	},
	"db.r5.16xlarge": map[string]int64{
		"default":          43690,
		"default.mysql5.7": 43690,
	},
	"db.m6g.large": map[string]int64{
		"default":            901,
		"default.postgres12": 901,
		"default.postgres13": 901,
		"default.postgres14": 901,
	},
	"db.m6g.xlarge": map[string]int64{
		"default":            1705,
		"default.postgres10": 1705,
		"default.postgres11": 1705,
		"default.postgres12": 1705,
		"default.postgres13": 1705,
		"default.postgres14": 1705,
	},
	"db.m6g.2xlarge": map[string]int64{
		"default":            3410,
		"default.postgres10": 3410,
		"default.postgres11": 3410,
		"default.postgres12": 3410,
		"default.postgres13": 3410,
		"default.postgres14": 3410,
	},
	"db.m6g.4xlarge": map[string]int64{
		"default":            5000,
		"default.postgres10": 5000,
		"default.postgres11": 5000,
		"default.postgres12": 5000,
		"default.postgres13": 5000,
		"default.postgres14": 5000,
	},
	"db.m6g.8xlarge": map[string]int64{
		"default":            5000,
		"default.postgres10": 5000,
		"default.postgres11": 5000,
		"default.postgres12": 5000,
		"default.postgres13": 5000,
		"default.postgres14": 5000,
	},
}

// RDSExporter defines an instance of the RDS Exporter
type RDSExporter struct {
	sess                       *session.Session
	AllocatedStorage           *prometheus.Desc
	DBInstanceClass            *prometheus.Desc
	DBInstanceStatus           *prometheus.Desc
	EngineVersion              *prometheus.Desc
	LatestRestorableTime       *prometheus.Desc
	MaxConnections             *prometheus.Desc
	MaxConnectionsMappingError *prometheus.Desc
	PendingMaintenanceActions  *prometheus.Desc
	PubliclyAccessible         *prometheus.Desc
	StorageEncrypted           *prometheus.Desc

	logger log.Logger
	mutex  *sync.Mutex
}

// NewRDSExporter creates a new RDSExporter instance
func NewRDSExporter(sess *session.Session, logger log.Logger) *RDSExporter {
	level.Info(logger).Log("msg", "Initializing RDS exporter")
	return &RDSExporter{
		sess:  sess,
		mutex: &sync.Mutex{},
		AllocatedStorage: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "rds_allocatedstorage"),
			"The amount of allocated storage in bytes.",
			[]string{"aws_region", "dbinstance_identifier"},
			nil,
		),
		DBInstanceClass: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "rds_dbinstanceclass"),
			"The DB instance class (type).",
			[]string{"aws_region", "dbinstance_identifier", "instance_class"},
			nil,
		),
		DBInstanceStatus: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "rds_dbinstancestatus"),
			"The instance status.",
			[]string{"aws_region", "dbinstance_identifier", "instance_status"},
			nil,
		),
		EngineVersion: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "rds_engineversion"),
			"The DB engine type and version.",
			[]string{"aws_region", "dbinstance_identifier", "engine", "engine_version"},
			nil,
		),
		LatestRestorableTime: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "rds_latestrestorabletime"),
			"Latest restorable time (UTC date timestamp).",
			[]string{"aws_region", "dbinstance_identifier"},
			nil,
		),
		MaxConnections: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "rds_maxconnections"),
			"The DB's max_connections value",
			[]string{"aws_region", "dbinstance_identifier"},
			nil,
		),
		MaxConnectionsMappingError: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "rds_maxconnections_error"),
			"Indicates no mapping found for instance/parameter group.",
			[]string{"aws_region", "dbinstance_identifier", "instance_class"},
			nil,
		),
		PendingMaintenanceActions: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "rds_pendingmaintenanceactions"),
			"Pending maintenance actions for a RDS instance. 0 indicates no available maintenance and a separate metric with a value of 1 will be published for every separate action.",
			[]string{"aws_region", "dbinstance_identifier", "action", "auto_apply_after", "current_apply_date", "description"},
			nil,
		),
		PubliclyAccessible: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "rds_publiclyaccessible"),
			"Indicates if the DB is publicly accessible",
			[]string{"aws_region", "dbinstance_identifier"},
			nil,
		),
		StorageEncrypted: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "rds_storageencrypted"),
			"Indicates if the DB storage is encrypted",
			[]string{"aws_region", "dbinstance_identifier"},
			nil,
		),
		logger: logger,
	}
}

// Describe is used by the Prometheus client to return a description of the metrics
func (e *RDSExporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.AllocatedStorage
	ch <- e.DBInstanceClass
	ch <- e.DBInstanceStatus
	ch <- e.EngineVersion
	ch <- e.LatestRestorableTime
	ch <- e.MaxConnections
	ch <- e.MaxConnectionsMappingError
	ch <- e.PendingMaintenanceActions
	ch <- e.PubliclyAccessible
	ch <- e.StorageEncrypted
}

// Collect is used by the Prometheus client to collect and return the metrics values
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
			level.Error(e.logger).Log("msg", "Call to DescribeDBInstances failed", "region", *e.sess.Config.Region, "err", err)
			exporterMetrics.IncrementErrors()
			return
		}
		instances = append(instances, result.DBInstances...)
		input.Marker = result.Marker
		if result.Marker == nil {
			break
		}
	}

	for _, instance := range instances {
		var maxConnections int64
		if valmap, ok := DBMaxConnections[*instance.DBInstanceClass]; ok {
			var maxconn int64
			var found bool
			if val, ok := valmap[*instance.DBParameterGroups[0].DBParameterGroupName]; ok {
				maxconn = val
				found = true
			} else if val, ok := valmap["default"]; ok {
				maxconn = val
				found = true
			}
			if found {
				level.Debug(e.logger).Log("msg", "Found mapping for instance",
					"type", *instance.DBInstanceClass,
					"group", *instance.DBParameterGroups[0].DBParameterGroupName,
					"value", maxconn)
				maxConnections = maxconn
				ch <- prometheus.MustNewConstMetric(e.MaxConnectionsMappingError, prometheus.GaugeValue, 0, *e.sess.Config.Region, *instance.DBInstanceIdentifier, *instance.DBInstanceClass)
			} else {
				level.Error(e.logger).Log("msg", "No DB max_connections mapping exists for instance",
					"type", *instance.DBInstanceClass,
					"group", *instance.DBParameterGroups[0].DBParameterGroupName)
				ch <- prometheus.MustNewConstMetric(e.MaxConnectionsMappingError, prometheus.GaugeValue, 1, *e.sess.Config.Region, *instance.DBInstanceIdentifier, *instance.DBInstanceClass)
			}
		} else {
			level.Error(e.logger).Log("msg", "No DB max_connections mapping exists for instance",
				"type", *instance.DBInstanceClass)
			ch <- prometheus.MustNewConstMetric(e.MaxConnectionsMappingError, prometheus.GaugeValue, 1, *e.sess.Config.Region, *instance.DBInstanceIdentifier, *instance.DBInstanceClass)
		}

		if *instance.PubliclyAccessible {
			ch <- prometheus.MustNewConstMetric(e.PubliclyAccessible, prometheus.GaugeValue, 1, *e.sess.Config.Region, *instance.DBInstanceIdentifier)

		} else {
			ch <- prometheus.MustNewConstMetric(e.PubliclyAccessible, prometheus.GaugeValue, 0, *e.sess.Config.Region, *instance.DBInstanceIdentifier)

		}

		if *instance.StorageEncrypted {
			ch <- prometheus.MustNewConstMetric(e.StorageEncrypted, prometheus.GaugeValue, 1, *e.sess.Config.Region, *instance.DBInstanceIdentifier)

		} else {
			ch <- prometheus.MustNewConstMetric(e.StorageEncrypted, prometheus.GaugeValue, 0, *e.sess.Config.Region, *instance.DBInstanceIdentifier)

		}

		if instance.LatestRestorableTime != nil {
			ch <- prometheus.MustNewConstMetric(e.LatestRestorableTime, prometheus.CounterValue, float64(instance.LatestRestorableTime.Unix()), *e.sess.Config.Region, *instance.DBInstanceIdentifier)
		} else {
			ch <- prometheus.MustNewConstMetric(e.LatestRestorableTime, prometheus.CounterValue, float64(0), *e.sess.Config.Region, *instance.DBInstanceIdentifier)
		}

		ch <- prometheus.MustNewConstMetric(e.MaxConnections, prometheus.GaugeValue, float64(maxConnections), *e.sess.Config.Region, *instance.DBInstanceIdentifier)
		ch <- prometheus.MustNewConstMetric(e.AllocatedStorage, prometheus.GaugeValue, float64(*instance.AllocatedStorage*1024*1024*1024), *e.sess.Config.Region, *instance.DBInstanceIdentifier)
		ch <- prometheus.MustNewConstMetric(e.DBInstanceStatus, prometheus.GaugeValue, 1, *e.sess.Config.Region, *instance.DBInstanceIdentifier, *instance.DBInstanceStatus)
		ch <- prometheus.MustNewConstMetric(e.EngineVersion, prometheus.GaugeValue, 1, *e.sess.Config.Region, *instance.DBInstanceIdentifier, *instance.Engine, *instance.EngineVersion)
		ch <- prometheus.MustNewConstMetric(e.DBInstanceClass, prometheus.GaugeValue, 1, *e.sess.Config.Region, *instance.DBInstanceIdentifier, *instance.DBInstanceClass)
	}

	// Get pending maintenance data because this isn't provided in DescribeDBInstances
	var instancesPendMaintActionsData []*rds.ResourcePendingMaintenanceActions
	describePendingMaintInput := &rds.DescribePendingMaintenanceActionsInput{}
	instancesWithPendingMaint := make(map[string]bool)

	for {
		exporterMetrics.IncrementRequests()
		result, err := svc.DescribePendingMaintenanceActions(describePendingMaintInput)
		if err != nil {
			level.Error(e.logger).Log("msg", "Call to DescribePendingMaintenanceActions failed", "region", *e.sess.Config.Region, "err", err)
			exporterMetrics.IncrementErrors()
			return
		}
		instancesPendMaintActionsData = append(instancesPendMaintActionsData, result.PendingMaintenanceActions...)
		describePendingMaintInput.Marker = result.Marker
		if result.Marker == nil {
			break
		}
	}

	// Create the metrics for all instances that have pending maintenance actions
	for _, instance := range instancesPendMaintActionsData {
		for _, action := range instance.PendingMaintenanceActionDetails {
			// DescribePendingMaintenanceActions only returns ARNs, so this gets the identifier.
			dbIdentifier := strings.Split(*instance.ResourceIdentifier, ":")[6]
			instancesWithPendingMaint[dbIdentifier] = true

			var autoApplyDate string
			if action.AutoAppliedAfterDate == nil {
				autoApplyDate = ""
			} else {
				autoApplyDate = action.AutoAppliedAfterDate.String()
			}

			var currentApplyDate string
			if action.CurrentApplyDate == nil {
				currentApplyDate = ""
			} else {
				currentApplyDate = action.CurrentApplyDate.String()
			}

			ch <- prometheus.MustNewConstMetric(e.PendingMaintenanceActions, prometheus.GaugeValue, 1, *e.sess.Config.Region, dbIdentifier, *action.Action, autoApplyDate, currentApplyDate, *action.Description)
		}
	}

	// DescribePendingMaintenanceActions only returns data about database with pending maintenance, so for any of the
	// other databases returned from DescribeDBInstances, publish a value of "0" indicating that maintenance isn't
	// available.
	for _, instance := range instances {
		if !instancesWithPendingMaint[*instance.DBInstanceIdentifier] {
			ch <- prometheus.MustNewConstMetric(e.PendingMaintenanceActions, prometheus.GaugeValue, 0, *e.sess.Config.Region, *instance.DBInstanceIdentifier, "", "", "", "")
		}
	}
}
