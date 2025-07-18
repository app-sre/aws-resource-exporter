package main

import (
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/app-sre/aws-resource-exporter/pkg"
	"github.com/app-sre/aws-resource-exporter/pkg/awsclient"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promslog"
	"github.com/prometheus/common/version"
)

const (
	namespace                      = "aws_resources_exporter"
	DEFAULT_TIMEOUT  time.Duration = 30 * time.Second
	CONFIG_FILE_PATH               = "./aws-resource-exporter-config.yaml"
)

var (
	listenAddress = kingpin.Flag("web.listen-address", "The address to listen on for HTTP requests.").Default(":9115").String()
	metricsPath   = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
)

func main() {
	os.Exit(run())
}

func getAwsAccountNumber(logger *slog.Logger, sess *session.Session) (string, error) {
	stsClient := sts.New(sess)
	identityOutput, err := stsClient.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		logger.Error("Could not retrieve caller identity of the aws account", "err", err)
		return "", err
	}
	return *identityOutput.Account, nil
}

func setupCollectors(logger *slog.Logger, configFile string) ([]prometheus.Collector, error) {
	var collectors []prometheus.Collector
	config, err := pkg.LoadExporterConfiguration(logger, configFile)
	if err != nil {
		return nil, err
	}
	logger.Info("Configuring vpc with regions", "regions", strings.Join(config.VpcConfig.Regions, ","))
	logger.Info("Configuring rds with regions", "regions", strings.Join(config.RdsConfig.Regions, ","))
	logger.Info("Configuring ec2 with regions", "regions", strings.Join(config.EC2Config.Regions, ","))
	logger.Info("Configuring route53 with region", "region", config.Route53Config.Region)
	logger.Info("Configuring elasticache with regions", "regions", strings.Join(config.ElastiCacheConfig.Regions, ","))
	logger.Info("Configuring msk with regions", "regions", strings.Join(config.MskConfig.Regions, ","))
	logger.Info("Will VPC metrics be gathered?", "vpc-enabled", config.VpcConfig.Enabled)
	logger.Info("Will IAM metrics be gathered?", "iam-enabled", config.IamConfig.Enabled)

	sessionRegion := "us-east-1"
	if sr := os.Getenv("AWS_REGION"); sr != "" {
		sessionRegion = sr
	}

	// Create a single session here, because we need the accountid, before we create the other configs
	awsConfig := aws.NewConfig().WithRegion(sessionRegion)
	sess := session.Must(session.NewSession(awsConfig))
	awsAccountId, err := getAwsAccountNumber(logger, sess)
	if err != nil {
		return collectors, err
	}
	var vpcSessions []*session.Session
	if config.VpcConfig.Enabled {
		for _, region := range config.VpcConfig.Regions {
			config := aws.NewConfig().WithRegion(region)
			sess := session.Must(session.NewSession(config))
			vpcSessions = append(vpcSessions, sess)
		}
		vpcExporter := pkg.NewVPCExporter(vpcSessions, logger, config.VpcConfig, awsAccountId)
		collectors = append(collectors, vpcExporter)
		go vpcExporter.CollectLoop()
	}
	logger.Info("Will RDS metrics be gathered?", "rds-enabled", config.RdsConfig.Enabled)
	var rdsSessions []*session.Session
	if config.RdsConfig.Enabled {
		for _, region := range config.RdsConfig.Regions {
			config := aws.NewConfig().WithRegion(region)
			sess := session.Must(session.NewSession(config))
			rdsSessions = append(rdsSessions, sess)
		}
		rdsExporter := pkg.NewRDSExporter(rdsSessions, logger, config.RdsConfig, awsAccountId)
		collectors = append(collectors, rdsExporter)
		go rdsExporter.CollectLoop()
	}
	logger.Info("Will EC2 metrics be gathered?", "ec2-enabled", config.EC2Config.Enabled)
	var ec2Sessions []*session.Session
	if config.EC2Config.Enabled {
		for _, region := range config.EC2Config.Regions {
			config := aws.NewConfig().WithRegion(region)
			sess := session.Must(session.NewSession(config))
			ec2Sessions = append(ec2Sessions, sess)
		}
		ec2Exporter := pkg.NewEC2Exporter(ec2Sessions, logger, config.EC2Config, awsAccountId)
		collectors = append(collectors, ec2Exporter)
		go ec2Exporter.CollectLoop()
	}
	logger.Info("Will Route53 metrics be gathered?", "route53-enabled", config.Route53Config.Enabled)
	if config.Route53Config.Enabled {
		awsConfig := aws.NewConfig().WithRegion(config.Route53Config.Region)
		sess := session.Must(session.NewSession(awsConfig))
		r53Exporter := pkg.NewRoute53Exporter(sess, logger, config.Route53Config, awsAccountId)
		collectors = append(collectors, r53Exporter)
		go r53Exporter.CollectLoop()
	}
	logger.Info("Will ElastiCache metrics be gathered?", "elasticache-enabled", config.ElastiCacheConfig.Enabled)
	var elasticacheSessions []*session.Session
	if config.ElastiCacheConfig.Enabled {
		for _, region := range config.ElastiCacheConfig.Regions {
			config := aws.NewConfig().WithRegion(region)
			sess := session.Must(session.NewSession(config))
			elasticacheSessions = append(elasticacheSessions, sess)
		}
		elasticacheExporter := pkg.NewElastiCacheExporter(elasticacheSessions, logger, config.ElastiCacheConfig, awsAccountId)
		collectors = append(collectors, elasticacheExporter)
		go elasticacheExporter.CollectLoop()
	}
	logger.Info("Will MSK metrics be gathered?", "msk-enabled", config.MskConfig.Enabled)
	var mskSessions []*session.Session
	if config.MskConfig.Enabled {
		for _, region := range config.MskConfig.Regions {
			config := aws.NewConfig().WithRegion(region)
			sess := session.Must(session.NewSession(config))
			mskSessions = append(mskSessions, sess)
		}
		mskExporter := pkg.NewMSKExporter(mskSessions, logger, config.MskConfig, awsAccountId)
		collectors = append(collectors, mskExporter)
		go mskExporter.CollectLoop()
	}
	logger.Info("Will IAM metrics be gathered?", "iam-enabled", config.IamConfig.Enabled)
	if config.IamConfig.Enabled {
		awsConfig := aws.NewConfig().WithRegion(config.IamConfig.Region) // IAM is global, this region just for AWS SDK initialization
		sess := session.Must(session.NewSession(awsConfig))
		iamExporter := pkg.NewIAMExporter(sess, logger, config.IamConfig, awsAccountId)
		collectors = append(collectors, iamExporter)
		go iamExporter.CollectLoop()
	}

	return collectors, nil
}

func run() int {
	promslogConfig := &promslog.Config{}

	kingpin.Version(version.Print(namespace))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
	logger := promslog.New(promslogConfig)

	logger.Info("Starting"+namespace, "version", version.Info())
	logger.Info("Build context", slog.String("context", version.BuildContext()))

	awsclient.AwsExporterMetrics = awsclient.NewExporterMetrics(namespace)

	var configFile string
	if path := os.Getenv("AWS_RESOURCE_EXPORTER_CONFIG_FILE"); path != "" {
		configFile = path
	} else {
		configFile = CONFIG_FILE_PATH
	}
	cs, err := setupCollectors(logger, configFile)
	if err != nil {
		logger.Error("Could not load configuration file", "err", err)
		return 1
	}
	collectors := append(cs, awsclient.AwsExporterMetrics)
	prometheus.MustRegister(
		collectors...,
	)

	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>AWS Resources Exporter</title></head>
             <body>
             <h1>AWS Resources Exporter</h1>
             <p><a href='` + *metricsPath + `'>Metrics</a></p>
             </body>
             </html>`))
	})

	srv := http.Server{Addr: *listenAddress}
	srvc := make(chan struct{})
	term := make(chan os.Signal, 1)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("Starting HTTP server", "address", *listenAddress)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			logger.Error("Error starting HTTP server", "err", err)
			close(srvc)
		}
	}()

	for {
		select {
		case <-term:
			logger.Info("Received SIGTERM, exiting gracefully...")
			return 0
		case <-srvc:
			return 1
		}
	}
}
