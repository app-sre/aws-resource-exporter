package pkg

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/app-sre/aws-resource-exporter/pkg/awsclient"
	"github.com/aws/aws-sdk-go-v2/aws"
	rds_types "github.com/aws/aws-sdk-go-v2/service/rds/types"
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

// Non Aurora: https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/CHAP_Limits.html#RDS_Limits.MaxConnections
// DBInstanceClassMemory in bytes: Memory (in GiB) * 1024 * 1024 * 1024
// Attention: DBInstanceClassMemory is the real memory available for the DB procoess and not all the instance memory!
// * postgres: LEAST({DBInstanceClassMemory_in_Bytes / 9531392},5000)
// * mysql: {DBInstanceClassMemory/12582880} - 50 and round down to the nearest hundreds.

// For MYSQL Aurora see: https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/AuroraMySQL.Managing.Performance.html

// Attention: use "default" for all Postgres versions (non-aurora)!
var DBMaxConnections = map[string]map[string]int64{
	//
	// Tx
	//
	"db.t3.micro": map[string]int64{
		// Memory: 1 GiB
		"default":          112,
		"default.mysql5.7": 45,
		"default.mysql8.0": 45,
	},
	"db.t3.small": map[string]int64{
		// Memory: 2 GiB
		"default":          225,
		"default.mysql5.7": 130,
		"default.mysql8.0": 130,
	},
	"db.t3.medium": map[string]int64{
		// Memory: 4 GiB
		"default":          450,
		"default.mysql5.7": 300,
		"default.mysql8.0": 300,
	},
	"db.t4g.micro": map[string]int64{
		// Memory: 1 GiB
		"default":          112,
		"default.mysql5.7": 45,
		"default.mysql8.0": 45,
	},
	"db.t4g.small": map[string]int64{
		// Memory: 2 GiB
		"default":          225,
		"default.mysql5.7": 130,
		"default.mysql8.0": 130,
	},
	"db.t4g.medium": map[string]int64{
		// Memory: 4 GiB
		"default":          450,
		"default.mysql5.7": 300,
		"default.mysql8.0": 300,
	},
	"db.t4g.large": map[string]int64{
		// Memory: 8 GiB
		"default":          900,
		"default.mysql5.7": 600,
		"default.mysql8.0": 600,
	},
	"db.t4g.xlarge": map[string]int64{
		// Memory: 16 GiB
		"default":          1800,
		"default.mysql5.7": 1300,
		"default.mysql8.0": 1300,
	},
	"db.t4g.2xlarge": map[string]int64{
		// Memory: 32 GiB
		"default":          3600,
		"default.mysql5.7": 2600,
		"default.mysql8.0": 2600,
	},

	//
	// M5
	//
	"db.m5.large": map[string]int64{
		// Memory: 8 GiB
		"default":          900,
		"default.mysql5.7": 600,
		"default.mysql8.0": 600,
	},
	"db.m5.xlarge": map[string]int64{
		// Memory: 16 GiB
		"default":          1800,
		"default.mysql5.7": 1300,
		"default.mysql8.0": 1300,
	},
	"db.m5.2xlarge": map[string]int64{
		// Memory: 32 GiB
		"default":          3600,
		"default.mysql5.7": 2600,
		"default.mysql8.0": 2600,
	},
	"db.m5.4xlarge": map[string]int64{
		// Memory: 64 GiB
		"default":          5000,
		"default.mysql5.7": 5300,
		"default.mysql8.0": 5300,
	},
	"db.m5.8xlarge": map[string]int64{
		// Memory: 128 GiB
		"default":          5000,
		"default.mysql5.7": 10700,
		"default.mysql8.0": 10700,
	},
	"db.m5.16xlarge": map[string]int64{
		// Memory: 256 GiB
		"default":                 5000,
		"default.aurora-mysql5.7": 6000,
		"default.aurora-mysql5.8": 6000,
		"default.aurora-mysql8.0": 6000,
		"default.mysql5.7":        21600,
		"default.mysql8.0":        21600,
	},

	//
	// M6g
	//
	"db.m6g.large": map[string]int64{
		// Memory: 8 GiB
		"default":          900,
		"default.mysql5.7": 600,
		"default.mysql8.0": 600,
	},
	"db.m6g.xlarge": map[string]int64{
		// Memory: 16 GiB
		"default":          1800,
		"default.mysql5.7": 1300,
		"default.mysql8.0": 1300,
	},
	"db.m6g.2xlarge": map[string]int64{
		// Memory: 32 GiB
		"default":          3600,
		"default.mysql5.7": 2600,
		"default.mysql8.0": 2600,
	},
	"db.m6g.4xlarge": map[string]int64{
		// Memory: 64 GiB
		"default":          5000,
		"default.mysql5.7": 5300,
		"default.mysql8.0": 5300,
	},
	"db.m6g.8xlarge": map[string]int64{
		// Memory: 128 GiB
		"default":          5000,
		"default.mysql5.7": 10700,
		"default.mysql8.0": 10700,
	},
	"db.m6g.12xlarge": map[string]int64{
		// Memory: 192 GiB
		"default":          5000,
		"default.mysql5.7": 16200,
		"default.mysql8.0": 16200,
	},

	//
	// M6gd
	//
	"db.m6gd.xlarge": map[string]int64{
		// Memory: 16 GiB
		"default":          1800,
		"default.mysql5.7": 1300,
		"default.mysql8.0": 1300,
	},
	"db.m6gd.2xlarge": map[string]int64{
		// Memory: 32 GiB
		"default":          3600,
		"default.mysql5.7": 2600,
		"default.mysql8.0": 2600,
	},

	//
	// M6i
	//
	"db.m6i.2xlarge": map[string]int64{
		// Memory: 32 GiB
		"default":          3600,
		"default.mysql5.7": 2600,
		"default.mysql8.0": 2600,
	},

	//
	// M7g
	//
	"db.m7g.large": map[string]int64{
		// Memory: 8 GiB
		"default":          900,
		"default.mysql5.7": 600,
		"default.mysql8.0": 600,
	},
	"db.m7g.xlarge": map[string]int64{
		// Memory: 16 GiB
		"default":          1800,
		"default.mysql5.7": 1300,
		"default.mysql8.0": 1300,
	},
	"db.m7g.2xlarge": map[string]int64{
		// Memory: 32 GiB
		"default":          3600,
		"default.mysql5.7": 2600,
		"default.mysql8.0": 2600,
	},
	"db.m7g.4xlarge": map[string]int64{
		// Memory: 64 GiB
		"default":          5000,
		"default.mysql5.7": 5300,
		"default.mysql8.0": 5300,
	},
	"db.m7g.8xlarge": map[string]int64{
		// Memory: 128 GiB
		"default":          5000,
		"default.mysql5.7": 10700,
		"default.mysql8.0": 10700,
	},
	"db.m7g.12xlarge": map[string]int64{
		// Memory: 192 GiB
		"default":          5000,
		"default.mysql5.7": 16200,
		"default.mysql8.0": 16200,
	},

	//
	// R5
	//
	"db.r5.large": map[string]int64{
		// Memory: 16 GiB
		"default":          1800,
		"default.mysql5.7": 1300,
		"default.mysql8.0": 1300,
	},
	"db.r5.xlarge": map[string]int64{
		// Memory: 32 GiB
		"default":          3600,
		"default.mysql5.7": 2600,
		"default.mysql8.0": 2600,
	},
	"db.r5.2xlarge": map[string]int64{
		// Memory: 64 GiB
		"default":                 5000,
		"default.mysql5.7":        5300,
		"default.mysql8.0":        5300,
		"default.aurora-mysql5.7": 3000,
		"default.aurora-mysql5.8": 3000,
		"default.aurora-mysql8.0": 3000,
	},
	"db.r5.4xlarge": map[string]int64{
		// Memory: 128 GiB
		"default":          5000,
		"default.mysql5.7": 10700,
		"default.mysql8.0": 10700,
	},
	"db.r5.8xlarge": map[string]int64{
		// Memory: 256 GiB
		"default":          5000,
		"default.mysql5.7": 21600,
		"default.mysql8.0": 21600,
	},
	"db.r5.12xlarge": map[string]int64{
		// Memory: 384 GiB
		"default":          5000,
		"default.mysql5.7": 32768,
		"default.mysql8.0": 32768,
	},
	"db.r5.16xlarge": map[string]int64{
		// Memory: 512 GiB
		"default":          5000,
		"default.mysql5.7": 43400,
		"default.mysql8.0": 43400,
	},
	"db.r5.24xlarge": map[string]int64{
		// Memory: 768 GiB
		"default":          5000,
		"default.mysql5.7": 65400,
		"default.mysql8.0": 65400,
	},

	//
	// R6
	//
	"db.r6g.12xlarge": map[string]int64{
		// Memory: 384 GiB
		"default":          5000,
		"default.mysql5.7": 32768,
		"default.mysql8.0": 32768,
	},
	"db.r6i.large": map[string]int64{
		// Memory: 16 GiB
		"default":          1800,
		"default.mysql5.7": 1300,
		"default.mysql8.0": 1300,
	},
	"db.r6g.xlarge": map[string]int64{
		// Memory: 32 GiB
		"default": 3484,
	},
	"db.r6i.16xlarge": map[string]int64{
		// Memory: 512 GiB
		"default":          5000,
		"default.mysql5.7": 43400,
		"default.mysql8.0": 43400,
	},
	"db.r6g.8xlarge": map[string]int64{
		// Memory: 256 GiB
		"default":          5000,
		"default.mysql5.7": 21600,
		"default.mysql8.0": 21600,
	},

	//
	// R7
	//
	"db.r7g.large": map[string]int64{
		// Memory: 16 GiB
		"default":          1800,
		"default.mysql5.7": 1300,
		"default.mysql8.0": 1300,
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
	[]string{"aws_region", "dbinstance_identifier", "engine", "engine_version", "aws_account_id"},
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
var EOLInfos *prometheus.Desc = prometheus.NewDesc(
	prometheus.BuildFQName(namespace, "", "rds_eol_info"),
	"The EOL date and status for the DB engine type and version.",
	[]string{"aws_region", "dbinstance_identifier", "engine", "engine_version", "eol_date", "eol_status"},
	nil,
)

// RDSExporter defines an instance of the RDS Exporter
type RDSExporter struct {
	configs      []aws.Config
	svcs         []awsclient.Client
	eolInfos     []EOLInfo
	thresholds   []Threshold
	awsAccountId string

	workers        int
	logsMetricsTTL int

	logger   *slog.Logger
	cache    MetricsCache
	interval time.Duration
	timeout  time.Duration
}

// NewRDSExporter creates a new RDSExporter instance
func NewRDSExporter(configs []aws.Config, logger *slog.Logger, config RDSConfig, awsAccountId string) *RDSExporter {
	logger.Info("Initializing RDS exporter")

	workers, _ := GetEnvIntValue(RDS_LOGS_METRICS_WORKERS)
	if workers == nil {
		workers = &RDS_LOGS_METRICS_WORKERS_DEFAULT
		logger.Info(fmt.Sprintf("Using default value for number Workers: %d", RDS_LOGS_METRICS_WORKERS_DEFAULT))
	} else {
		logger.Info(fmt.Sprintf("Using Env value for number of Workers: %d", *workers))
	}

	logMetricsTTL, _ := GetEnvIntValue(RDS_LOGS_METRICS_TTL)
	if logMetricsTTL == nil {
		logMetricsTTL = &RDS_LOGS_METRICS_TTL_DEFAULT
		logger.Info(fmt.Sprintf("Using default value for logs metrics TTL: %d", RDS_LOGS_METRICS_TTL_DEFAULT))
	} else {
		logger.Info(fmt.Sprintf("Using Env value for logs metrics TTL: %d", *logMetricsTTL))
	}
	var rdses []awsclient.Client
	for _, cfg := range configs {
		rdses = append(rdses, awsclient.NewClientFromConfig(cfg))
	}

	return &RDSExporter{
		configs:        configs,
		svcs:           rdses,
		workers:        *workers,
		logsMetricsTTL: *logMetricsTTL,
		logger:         logger,
		cache:          *NewMetricsCache(*config.CacheTTL),
		interval:       *config.Interval,
		timeout:        *config.Timeout,
		eolInfos:       config.EOLInfos,
		thresholds:     config.Thresholds,
		awsAccountId:   awsAccountId,
	}

}

func (e *RDSExporter) getRegion(configIndex int) string {
	return e.configs[configIndex].Region
}

func (e *RDSExporter) requestRDSLogMetrics(ctx context.Context, configIndex int, instanceId string) (*RDSLogsMetrics, error) {
	var logMetrics = &RDSLogsMetrics{
		logs:         0,
		totalLogSize: 0,
	}

	logOutPuts, err := e.svcs[configIndex].DescribeDBLogFilesAll(ctx, instanceId)
	if err != nil {
		e.logger.Error("Call to DescribeDBLogFiles failed",
			slog.String("region", e.getRegion(configIndex)),
			slog.String("instance", instanceId),
			slog.Any("err", err))
		awsclient.AwsExporterMetrics.IncrementErrors()
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

func (e *RDSExporter) addRDSLogMetrics(ctx context.Context, configIndex int, instanceId string) error {
	instaceLogFilesId := instanceId + "-" + "logfiles"
	var logMetrics *RDSLogsMetrics
	cachedItem, err := metricsProxy.GetMetricById(instaceLogFilesId)
	if err != nil {
		logMetrics, err = e.requestRDSLogMetrics(ctx, configIndex, instanceId)
		if err != nil {
			return err
		}
		metricsProxy.StoreMetricById(instaceLogFilesId, logMetrics, e.logsMetricsTTL)
	} else {
		logMetrics = cachedItem.value.(*RDSLogsMetrics)
	}
	e.cache.AddMetric(prometheus.MustNewConstMetric(LogsAmount, prometheus.GaugeValue, float64(logMetrics.logs), e.getRegion(configIndex), instanceId))
	e.cache.AddMetric(prometheus.MustNewConstMetric(LogsStorageSize, prometheus.GaugeValue, float64(logMetrics.totalLogSize), e.getRegion(configIndex), instanceId))
	return nil
}

func (e *RDSExporter) addAllLogMetrics(ctx context.Context, configIndex int, instances []rds_types.DBInstance) {
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
			e.addRDSLogMetrics(ctx, configIndex, instanceName)
		}(*instance.DBInstanceIdentifier)
	}
	wg.Wait()
}

func (e *RDSExporter) addAllInstanceMetrics(configIndex int, instances []rds_types.DBInstance, eolInfos []EOLInfo) {
	var eolMap = make(map[EOLKey]EOLInfo)

	// Fill eolMap with EOLInfo indexed by engine and version
	for _, eolinfo := range eolInfos {
		eolMap[EOLKey{Engine: eolinfo.Engine, Version: eolinfo.Version}] = eolinfo
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
				e.logger.Debug("Found mapping for instance",
					slog.String("type", *instance.DBInstanceClass),
					slog.String("group", *instance.DBParameterGroups[0].DBParameterGroupName),
					slog.Int64("value", maxconn))

				maxConnections = maxconn
				e.cache.AddMetric(prometheus.MustNewConstMetric(MaxConnectionsMappingError, prometheus.GaugeValue, 0, e.getRegion(configIndex), *instance.DBInstanceIdentifier, *instance.DBInstanceClass))
			} else {
				e.logger.Error("No DB max_connections mapping exists for instance",
					slog.String("type", *instance.DBInstanceClass),
					slog.String("group", *instance.DBParameterGroups[0].DBParameterGroupName))
				e.cache.AddMetric(prometheus.MustNewConstMetric(MaxConnectionsMappingError, prometheus.GaugeValue, 1, e.getRegion(configIndex), *instance.DBInstanceIdentifier, *instance.DBInstanceClass))
			}
		} else {
			e.logger.Error("No DB max_connections mapping exists for instance",
				slog.String("type", *instance.DBInstanceClass))
			e.cache.AddMetric(prometheus.MustNewConstMetric(MaxConnectionsMappingError, prometheus.GaugeValue, 1, e.getRegion(configIndex), *instance.DBInstanceIdentifier, *instance.DBInstanceClass))
		}

		//Gets EOL for engine and version
		if eolInfo, ok := eolMap[EOLKey{Engine: *instance.Engine, Version: *instance.EngineVersion}]; ok {
			eolStatus, err := GetEOLStatus(eolInfo.EOL, e.thresholds)
			if err != nil {
				e.logger.Error("Could not get days to RDS EOL for engine version",
					slog.String("engine", *instance.Engine),
					slog.String("version", *instance.EngineVersion),
					slog.Any("error", err))

			} else {
				e.cache.AddMetric(prometheus.MustNewConstMetric(EOLInfos, prometheus.GaugeValue, 1, e.getRegion(configIndex), *instance.DBInstanceIdentifier, *instance.Engine, *instance.EngineVersion, eolInfo.EOL, eolStatus))
			}
		} else {
			e.logger.Info("RDS EOL not found for engine version",
				slog.String("engine", *instance.Engine),
				slog.String("version", *instance.EngineVersion))
		}

		var public = 0.0
		if *instance.PubliclyAccessible {
			public = 1.0
		}
		e.cache.AddMetric(prometheus.MustNewConstMetric(PubliclyAccessible, prometheus.GaugeValue, public, e.getRegion(configIndex), *instance.DBInstanceIdentifier))

		var encrypted = 0.0
		if *instance.StorageEncrypted {
			encrypted = 1.0
		}
		e.cache.AddMetric(prometheus.MustNewConstMetric(StorageEncrypted, prometheus.GaugeValue, encrypted, e.getRegion(configIndex), *instance.DBInstanceIdentifier))

		var restoreTime = 0.0
		if instance.LatestRestorableTime != nil {
			restoreTime = float64(instance.LatestRestorableTime.Unix())
		}
		e.cache.AddMetric(prometheus.MustNewConstMetric(LatestRestorableTime, prometheus.CounterValue, restoreTime, e.getRegion(configIndex), *instance.DBInstanceIdentifier))

		e.cache.AddMetric(prometheus.MustNewConstMetric(MaxConnections, prometheus.GaugeValue, float64(maxConnections), e.getRegion(configIndex), *instance.DBInstanceIdentifier))
		e.cache.AddMetric(prometheus.MustNewConstMetric(AllocatedStorage, prometheus.GaugeValue, float64(int64(*instance.AllocatedStorage)*1024*1024*1024), e.getRegion(configIndex), *instance.DBInstanceIdentifier))
		e.cache.AddMetric(prometheus.MustNewConstMetric(DBInstanceStatus, prometheus.GaugeValue, 1, e.getRegion(configIndex), *instance.DBInstanceIdentifier, *instance.DBInstanceStatus))
		e.cache.AddMetric(prometheus.MustNewConstMetric(EngineVersion, prometheus.GaugeValue, 1, e.getRegion(configIndex), *instance.DBInstanceIdentifier, *instance.Engine, *instance.EngineVersion, e.awsAccountId))
		e.cache.AddMetric(prometheus.MustNewConstMetric(DBInstanceClass, prometheus.GaugeValue, 1, e.getRegion(configIndex), *instance.DBInstanceIdentifier, *instance.DBInstanceClass))
	}
}

func (e *RDSExporter) addAllPendingMaintenancesMetrics(ctx context.Context, configIndex int, instances []rds_types.DBInstance) {
	// Get pending maintenance data because this isn't provided in DescribeDBInstances
	instancesWithPendingMaint := make(map[string]bool)

	instancesPendMaintActionsData, err := e.svcs[configIndex].DescribePendingMaintenanceActionsAll(ctx)

	if err != nil {
		e.logger.Error("Call to DescribePendingMaintenanceActions failed",
			slog.String("region", e.getRegion(configIndex)),
			slog.Any("err", err))
		awsclient.AwsExporterMetrics.IncrementErrors()
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

			e.cache.AddMetric(prometheus.MustNewConstMetric(PendingMaintenanceActions, prometheus.GaugeValue, 1, e.getRegion(configIndex), dbIdentifier, *action.Action, autoApplyDate, currentApplyDate, *action.Description))
		}
	}

	// DescribePendingMaintenanceActions only returns data about database with pending maintenance, so for any of the
	// other databases returned from DescribeDBInstances, publish a value of "0" indicating that maintenance isn't
	// available.
	for _, instance := range instances {
		if !instancesWithPendingMaint[*instance.DBInstanceIdentifier] {
			e.cache.AddMetric(prometheus.MustNewConstMetric(PendingMaintenanceActions, prometheus.GaugeValue, 0, e.getRegion(configIndex), *instance.DBInstanceIdentifier, "", "", "", ""))
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
	ch <- EOLInfos

}

func (e *RDSExporter) CollectLoop() {
	for {
		ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
		for i, _ := range e.configs {

			instances, err := e.svcs[i].DescribeDBInstancesAll(ctx)
			if err != nil {
				e.logger.Error("Call to DescribeDBInstances failed",
					slog.String("region", e.getRegion(i)),
					slog.Any("err", err))
				awsclient.AwsExporterMetrics.IncrementErrors()
			}

			wg := sync.WaitGroup{}
			wg.Add(3)

			go func() {
				e.addAllInstanceMetrics(i, instances, e.eolInfos)
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

		e.logger.Info("RDS metrics Updated")

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
