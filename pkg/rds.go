package pkg

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/app-sre/aws-resource-exporter/pkg/awsclient"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

// Default TTL value for RDS logs related metrics
// To get the log metrics an api call for each instance is needed
// Since this cause rate limit problems to the AWS api, these metrics
// are cached for this amount of time before requesting them again
var RDS_LOGS_METRICS_TTL = "LOGS_METRICS_TTL"
var RDS_LOGS_METRICS_TTL_DEFAULT = 300

// RDS log metrics are requested in parallel with a workerPool.
// this variable sets the number of workers
var RDS_LOGS_METRICS_WORKERS = "LOGS_METRICS_WORKERS"
var RDS_LOGS_METRICS_WORKERS_DEFAULT = 10

// Struct to store RDS Instances log files data
// This struct is used to store the data in the MetricsProxy
type RDSLogsMetrics struct {
	logs         int
	totalLogSize int64
}

// MetricsProxy
var metricsProxy = NewMetricProxy()

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
	"db.r5.12xlarge": map[string]int64{
		"default":          16000,
		"default.mysql5.7": 16000,
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
	"db.m6i.2xlarge": map[string]int64{
		"default":            3410,
		"default.postgres10": 3410,
		"default.postgres11": 3410,
		"default.postgres12": 3410,
		"default.postgres13": 3410,
		"default.postgres14": 3410,
	},
	"db.m7g.large": map[string]int64{
		"default":            901,
		"default.postgres12": 901,
		"default.postgres13": 901,
		"default.postgres14": 901,
	},
	"db.m7g.xlarge": map[string]int64{
		"default":            1705,
		"default.postgres10": 1705,
		"default.postgres11": 1705,
		"default.postgres12": 1705,
		"default.postgres13": 1705,
		"default.postgres14": 1705,
	},
	"db.m7g.2xlarge": map[string]int64{
		"default":            3410,
		"default.postgres10": 3410,
		"default.postgres11": 3410,
		"default.postgres12": 3410,
		"default.postgres13": 3410,
		"default.postgres14": 3410,
	},
	"db.m7g.4xlarge": map[string]int64{
		"default":            5000,
		"default.postgres10": 5000,
		"default.postgres11": 5000,
		"default.postgres12": 5000,
		"default.postgres13": 5000,
		"default.postgres14": 5000,
	},
	"db.m7g.8xlarge": map[string]int64{
		"default":            5000,
		"default.postgres10": 5000,
		"default.postgres11": 5000,
		"default.postgres12": 5000,
		"default.postgres13": 5000,
		"default.postgres14": 5000,
	},
}

var AllocatedStorage *prometheus.Desc = prometheus.NewDesc(
	prometheus.BuildFQName(namespace, "", "rds_allocatedstorage"),
	"The amount of allocated storage in bytes.",
	[]string{"aws_region", "dbinstance_identifier"},
	nil,
)
var DBInstanceClass *prometheus.Desc = prometheus.NewDesc(
	prometheus.BuildFQName(namespace, "", "rds_dbinstanceclass"),
	"The DB instance class (type).",
	[]string{"aws_region", "dbinstance_identifier", "instance_class"},
	nil,
)
var DBInstanceStatus *prometheus.Desc = prometheus.NewDesc(
	prometheus.BuildFQName(namespace, "", "rds_dbinstancestatus"),
	"The instance status.",
	[]string{"aws_region", "dbinstance_identifier", "instance_status"},
	nil,
)
var EngineVersion *prometheus.Desc = prometheus.NewDesc(
	prometheus.BuildFQName(namespace, "", "rds_engineversion"),
	"The DB engine type and version.",
	[]string{"aws_region", "dbinstance_identifier", "engine", "engine_version"},
	nil,
)
var LatestRestorableTime *prometheus.Desc = prometheus.NewDesc(
	prometheus.BuildFQName(namespace, "", "rds_latestrestorabletime"),
	"Latest restorable time (UTC date timestamp).",
	[]string{"aws_region", "dbinstance_identifier"},
	nil,
)
var MaxConnections *prometheus.Desc = prometheus.NewDesc(
	prometheus.BuildFQName(namespace, "", "rds_maxconnections"),
	"The DB's max_connections value",
	[]string{"aws_region", "dbinstance_identifier"},
	nil,
)
var MaxConnectionsMappingError *prometheus.Desc = prometheus.NewDesc(
	prometheus.BuildFQName(namespace, "", "rds_maxconnections_error"),
	"Indicates no mapping found for instance/parameter group.",
	[]string{"aws_region", "dbinstance_identifier", "instance_class"},
	nil,
)
var PendingMaintenanceActions *prometheus.Desc = prometheus.NewDesc(
	prometheus.BuildFQName(namespace, "", "rds_pendingmaintenanceactions"),
	"Pending maintenance actions for a RDS instance. 0 indicates no available maintenance and a separate metric with a value of 1 will be published for every separate action.",
	[]string{"aws_region", "dbinstance_identifier", "action", "auto_apply_after", "current_apply_date", "description"},
	nil,
)
var PubliclyAccessible *prometheus.Desc = prometheus.NewDesc(
	prometheus.BuildFQName(namespace, "", "rds_publiclyaccessible"),
	"Indicates if the DB is publicly accessible",
	[]string{"aws_region", "dbinstance_identifier"},
	nil,
)
var StorageEncrypted *prometheus.Desc = prometheus.NewDesc(
	prometheus.BuildFQName(namespace, "", "rds_storageencrypted"),
	"Indicates if the DB storage is encrypted",
	[]string{"aws_region", "dbinstance_identifier"},
	nil,
)
var LogsStorageSize *prometheus.Desc = prometheus.NewDesc(
	prometheus.BuildFQName(namespace, "", "rds_logsstorage_size_bytes"),
	"The amount of storage consumed by log files (in bytes)",
	[]string{"aws_region", "dbinstance_identifier"},
	nil,
)
var LogsAmount *prometheus.Desc = prometheus.NewDesc(
	prometheus.BuildFQName(namespace, "", "rds_logs_amount"),
	"The amount of existent log files",
	[]string{"aws_region", "dbinstance_identifier"},
	nil,
)

// RDSExporter defines an instance of the RDS Exporter
type RDSExporter struct {
	sessions []*session.Session
	svcs     []awsclient.Client

	workers        int
	logsMetricsTTL int

	logger   log.Logger
	cache    MetricsCache
	interval time.Duration
	timeout  time.Duration
}

// NewRDSExporter creates a new RDSExporter instance
func NewRDSExporter(sessions []*session.Session, logger log.Logger, config RDSConfig) *RDSExporter {
	level.Info(logger).Log("msg", "Initializing RDS exporter")

	workers, _ := GetEnvIntValue(RDS_LOGS_METRICS_WORKERS)
	if workers == nil {
		workers = &RDS_LOGS_METRICS_WORKERS_DEFAULT
		level.Info(logger).Log("msg", fmt.Sprintf("Using default value for number Workers: %d", RDS_LOGS_METRICS_WORKERS_DEFAULT))
	} else {
		level.Info(logger).Log("msg", fmt.Sprintf("Using Env value for number of Workers: %d", *workers))
	}

	logMetricsTTL, _ := GetEnvIntValue(RDS_LOGS_METRICS_TTL)
	if logMetricsTTL == nil {
		logMetricsTTL = &RDS_LOGS_METRICS_TTL_DEFAULT
		level.Info(logger).Log("msg", fmt.Sprintf("Using default value for logs metrics TTL: %d", RDS_LOGS_METRICS_TTL_DEFAULT))
	} else {
		level.Info(logger).Log("msg", fmt.Sprintf("Using Env value for logs metrics TTL: %d", *logMetricsTTL))
	}
	var rdses []awsclient.Client
	for _, session := range sessions {
		rdses = append(rdses, awsclient.NewClientFromSession(session))
	}

	return &RDSExporter{
		sessions:       sessions,
		svcs:           rdses,
		workers:        *workers,
		logsMetricsTTL: *logMetricsTTL,
		logger:         logger,
		cache:          *NewMetricsCache(*config.CacheTTL),
		interval:       *config.Interval,
		timeout:        *config.Timeout,
	}
}

func (e *RDSExporter) getRegion(sessionIndex int) string {
	return *e.sessions[sessionIndex].Config.Region
}

func (e *RDSExporter) requestRDSLogMetrics(ctx context.Context, sessionIndex int, instanceId string) (*RDSLogsMetrics, error) {
	var logMetrics = &RDSLogsMetrics{
		logs:         0,
		totalLogSize: 0,
	}

	logOutPuts, err := e.svcs[sessionIndex].DescribeDBLogFilesAll(ctx, instanceId)
	if err != nil {
		level.Error(e.logger).Log("msg", "Call to DescribeDBLogFiles failed", "region", e.getRegion(sessionIndex), "instance", &instanceId, "err", err)
		return nil, err
	}

	for _, outputs := range logOutPuts {
		for _, log := range outputs.DescribeDBLogFiles {
			logMetrics.logs++
			logMetrics.totalLogSize += *log.Size
		}

	}

	return logMetrics, nil
}

func (e *RDSExporter) addRDSLogMetrics(ctx context.Context, sessionIndex int, instanceId string) error {
	instaceLogFilesId := instanceId + "-" + "logfiles"
	var logMetrics *RDSLogsMetrics
	cachedItem, err := metricsProxy.GetMetricById(instaceLogFilesId)
	if err != nil {
		logMetrics, err = e.requestRDSLogMetrics(ctx, sessionIndex, instanceId)
		if err != nil {
			return err
		}
		metricsProxy.StoreMetricById(instaceLogFilesId, logMetrics, e.logsMetricsTTL)
	} else {
		logMetrics = cachedItem.value.(*RDSLogsMetrics)
	}
	e.cache.AddMetric(prometheus.MustNewConstMetric(LogsAmount, prometheus.GaugeValue, float64(logMetrics.logs), e.getRegion(sessionIndex), instanceId))
	e.cache.AddMetric(prometheus.MustNewConstMetric(LogsStorageSize, prometheus.GaugeValue, float64(logMetrics.totalLogSize), e.getRegion(sessionIndex), instanceId))
	return nil
}

func (e *RDSExporter) addAllLogMetrics(ctx context.Context, sessionIndex int, instances []*rds.DBInstance) {
	wg := &sync.WaitGroup{}
	wg.Add(len(instances))

	// this channel is used to limit the number of concurrency
	sem := make(chan int, e.workers)

	defer close(sem)
	for _, instance := range instances {
		sem <- 1
		go func(instanceName string) {
			defer func() {
				<-sem
				wg.Done()
			}()
			e.addRDSLogMetrics(ctx, sessionIndex, instanceName)
		}(*instance.DBInstanceIdentifier)
	}
	wg.Wait()
}

func (e *RDSExporter) addAllInstanceMetrics(sessionIndex int, instances []*rds.DBInstance) {
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
				e.cache.AddMetric(prometheus.MustNewConstMetric(MaxConnectionsMappingError, prometheus.GaugeValue, 0, e.getRegion(sessionIndex), *instance.DBInstanceIdentifier, *instance.DBInstanceClass))
			} else {
				level.Error(e.logger).Log("msg", "No DB max_connections mapping exists for instance",
					"type", *instance.DBInstanceClass,
					"group", *instance.DBParameterGroups[0].DBParameterGroupName)
				e.cache.AddMetric(prometheus.MustNewConstMetric(MaxConnectionsMappingError, prometheus.GaugeValue, 1, e.getRegion(sessionIndex), *instance.DBInstanceIdentifier, *instance.DBInstanceClass))
			}
		} else {
			level.Error(e.logger).Log("msg", "No DB max_connections mapping exists for instance",
				"type", *instance.DBInstanceClass)
			e.cache.AddMetric(prometheus.MustNewConstMetric(MaxConnectionsMappingError, prometheus.GaugeValue, 1, e.getRegion(sessionIndex), *instance.DBInstanceIdentifier, *instance.DBInstanceClass))
		}

		var public = 0.0
		if *instance.PubliclyAccessible {
			public = 1.0
		}
		e.cache.AddMetric(prometheus.MustNewConstMetric(PubliclyAccessible, prometheus.GaugeValue, public, e.getRegion(sessionIndex), *instance.DBInstanceIdentifier))

		var encrypted = 0.0
		if *instance.StorageEncrypted {
			encrypted = 1.0
		}
		e.cache.AddMetric(prometheus.MustNewConstMetric(StorageEncrypted, prometheus.GaugeValue, encrypted, e.getRegion(sessionIndex), *instance.DBInstanceIdentifier))

		var restoreTime = 0.0
		if instance.LatestRestorableTime != nil {
			restoreTime = float64(instance.LatestRestorableTime.Unix())
		}
		e.cache.AddMetric(prometheus.MustNewConstMetric(LatestRestorableTime, prometheus.CounterValue, restoreTime, e.getRegion(sessionIndex), *instance.DBInstanceIdentifier))

		e.cache.AddMetric(prometheus.MustNewConstMetric(MaxConnections, prometheus.GaugeValue, float64(maxConnections), e.getRegion(sessionIndex), *instance.DBInstanceIdentifier))
		e.cache.AddMetric(prometheus.MustNewConstMetric(AllocatedStorage, prometheus.GaugeValue, float64(*instance.AllocatedStorage*1024*1024*1024), e.getRegion(sessionIndex), *instance.DBInstanceIdentifier))
		e.cache.AddMetric(prometheus.MustNewConstMetric(DBInstanceStatus, prometheus.GaugeValue, 1, e.getRegion(sessionIndex), *instance.DBInstanceIdentifier, *instance.DBInstanceStatus))
		e.cache.AddMetric(prometheus.MustNewConstMetric(EngineVersion, prometheus.GaugeValue, 1, e.getRegion(sessionIndex), *instance.DBInstanceIdentifier, *instance.Engine, *instance.EngineVersion))
		e.cache.AddMetric(prometheus.MustNewConstMetric(DBInstanceClass, prometheus.GaugeValue, 1, e.getRegion(sessionIndex), *instance.DBInstanceIdentifier, *instance.DBInstanceClass))
	}
}

func (e *RDSExporter) addAllPendingMaintenancesMetrics(ctx context.Context, sessionIndex int, instances []*rds.DBInstance) {
	// Get pending maintenance data because this isn't provided in DescribeDBInstances
	instancesWithPendingMaint := make(map[string]bool)

	instancesPendMaintActionsData, err := e.svcs[sessionIndex].DescribePendingMaintenanceActionsAll(ctx)

	if err != nil {
		level.Error(e.logger).Log("msg", "Call to DescribePendingMaintenanceActions failed", "region", e.getRegion(sessionIndex), "err", err)
		return
	}

	// Create the metrics for all instances that have pending maintenance actions
	for _, instance := range instancesPendMaintActionsData {
		for _, action := range instance.PendingMaintenanceActionDetails {
			// DescribePendingMaintenanceActions only returns ARNs, so this gets the identifier.
			dbIdentifier := strings.Split(*instance.ResourceIdentifier, ":")[6]
			instancesWithPendingMaint[dbIdentifier] = true

			var autoApplyDate string
			if action.AutoAppliedAfterDate != nil {
				autoApplyDate = action.AutoAppliedAfterDate.String()
			}

			var currentApplyDate string
			if action.CurrentApplyDate != nil {
				currentApplyDate = action.CurrentApplyDate.String()
			}

			e.cache.AddMetric(prometheus.MustNewConstMetric(PendingMaintenanceActions, prometheus.GaugeValue, 1, e.getRegion(sessionIndex), dbIdentifier, *action.Action, autoApplyDate, currentApplyDate, *action.Description))
		}
	}

	// DescribePendingMaintenanceActions only returns data about database with pending maintenance, so for any of the
	// other databases returned from DescribeDBInstances, publish a value of "0" indicating that maintenance isn't
	// available.
	for _, instance := range instances {
		if !instancesWithPendingMaint[*instance.DBInstanceIdentifier] {
			e.cache.AddMetric(prometheus.MustNewConstMetric(PendingMaintenanceActions, prometheus.GaugeValue, 0, e.getRegion(sessionIndex), *instance.DBInstanceIdentifier, "", "", "", ""))
		}
	}

}

// Describe is used by the Prometheus client to return a description of the metrics
func (e *RDSExporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- AllocatedStorage
	ch <- DBInstanceClass
	ch <- DBInstanceStatus
	ch <- EngineVersion
	ch <- LatestRestorableTime
	ch <- MaxConnections
	ch <- MaxConnectionsMappingError
	ch <- PendingMaintenanceActions
	ch <- PubliclyAccessible
	ch <- StorageEncrypted
}

func (e *RDSExporter) CollectLoop() {
	for {
		ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
		for i, _ := range e.sessions {

			instances, err := e.svcs[i].DescribeDBInstancesAll(ctx)
			if err != nil {
				level.Error(e.logger).Log("msg", "Call to DescribeDBInstances failed", "region", *e.sessions[i].Config.Region, "err", err)
			}

			wg := sync.WaitGroup{}
			wg.Add(3)

			go func() {
				e.addAllInstanceMetrics(i, instances)
				wg.Done()
			}()
			go func() {
				e.addAllLogMetrics(ctx, i, instances)
				wg.Done()
			}()
			go func() {
				e.addAllPendingMaintenancesMetrics(ctx, i, instances)
				wg.Done()
			}()
			wg.Wait()
		}

		level.Info(e.logger).Log("msg", "RDS metrics Updated")

		cancel()
		time.Sleep(e.interval)
	}
}

// Collect is used by the Prometheus client to collect and return the metrics values
func (e *RDSExporter) Collect(ch chan<- prometheus.Metric) {
	for _, m := range e.cache.GetAllMetrics() {
		ch <- m
	}
}
