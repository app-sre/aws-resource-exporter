package main

import (
	"context"
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
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
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

func getAwsAccountNumber(logger *slog.Logger, cfg aws.Config) (string, error) {
	stsClient := sts.NewFromConfig(cfg)
	identityOutput, err := stsClient.GetCallerIdentity(context.Background(), &sts.GetCallerIdentityInput{})
	if err != nil {
		logger.Error("Could not retrieve caller identity of the aws account", "err", err)
		return "", err
	}
	return *identityOutput.Account, nil
}

func setupCollectors(logger *slog.Logger, configFile string) ([]prometheus.Collector, error) {
	var collectors []prometheus.Collector
	exporterConfig, err := pkg.LoadExporterConfiguration(logger, configFile)
	if err != nil {
		return nil, err
	}
	logger.Info("Configuring vpc with regions", "regions", strings.Join(exporterConfig.VpcConfig.Regions, ","))
	logger.Info("Configuring rds with regions", "regions", strings.Join(exporterConfig.RdsConfig.Regions, ","))
	logger.Info("Configuring ec2 with regions", "regions", strings.Join(exporterConfig.EC2Config.Regions, ","))
	logger.Info("Configuring route53 with region", "region", exporterConfig.Route53Config.Region)
	logger.Info("Configuring elasticache with regions", "regions", strings.Join(exporterConfig.ElastiCacheConfig.Regions, ","))
	logger.Info("Configuring msk with regions", "regions", strings.Join(exporterConfig.MskConfig.Regions, ","))
	logger.Info("Will VPC metrics be gathered?", "vpc-enabled", exporterConfig.VpcConfig.Enabled)
	logger.Info("Will IAM metrics be gathered?", "iam-enabled", exporterConfig.IamConfig.Enabled)

	ctx := context.Background()
	sessionRegion := "us-east-1"
	if sr := os.Getenv("AWS_REGION"); sr != "" {
		sessionRegion = sr
	}

	// Create a single config here, because we need the accountid, before we create the other configs
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(sessionRegion))
	if err != nil {
		logger.Error("Could not load AWS config", "err", err)
		return collectors, err
	}
	awsAccountId, err := getAwsAccountNumber(logger, cfg)
	if err != nil {
		return collectors, err
	}
	var vpcConfigs []aws.Config
	if exporterConfig.VpcConfig.Enabled {
		for _, region := range exporterConfig.VpcConfig.Regions {
			regionCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
			if err != nil {
				logger.Error("Could not load AWS config for VPC", "region", region, "err", err)
				return collectors, err
			}
			vpcConfigs = append(vpcConfigs, regionCfg)
		}
		vpcExporter := pkg.NewVPCExporter(vpcConfigs, logger, exporterConfig.VpcConfig, awsAccountId)
		collectors = append(collectors, vpcExporter)
		go vpcExporter.CollectLoop()
	}
	logger.Info("Will RDS metrics be gathered?", "rds-enabled", exporterConfig.RdsConfig.Enabled)
	var rdsConfigs []aws.Config
	if exporterConfig.RdsConfig.Enabled {
		for _, region := range exporterConfig.RdsConfig.Regions {
			regionCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
			if err != nil {
				logger.Error("Could not load AWS config for RDS", "region", region, "err", err)
				return collectors, err
			}
			rdsConfigs = append(rdsConfigs, regionCfg)
		}
		rdsExporter := pkg.NewRDSExporter(rdsConfigs, logger, exporterConfig.RdsConfig, awsAccountId)
		collectors = append(collectors, rdsExporter)
		go rdsExporter.CollectLoop()
	}
	logger.Info("Will EC2 metrics be gathered?", "ec2-enabled", exporterConfig.EC2Config.Enabled)
	var ec2Configs []aws.Config
	if exporterConfig.EC2Config.Enabled {
		for _, region := range exporterConfig.EC2Config.Regions {
			regionCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
			if err != nil {
				logger.Error("Could not load AWS config for EC2", "region", region, "err", err)
				return collectors, err
			}
			ec2Configs = append(ec2Configs, regionCfg)
		}
		ec2Exporter := pkg.NewEC2Exporter(ec2Configs, logger, exporterConfig.EC2Config, awsAccountId)
		collectors = append(collectors, ec2Exporter)
		go ec2Exporter.CollectLoop()
	}
	logger.Info("Will Route53 metrics be gathered?", "route53-enabled", exporterConfig.Route53Config.Enabled)
	if exporterConfig.Route53Config.Enabled {
		regionCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(exporterConfig.Route53Config.Region))
		if err != nil {
			logger.Error("Could not load AWS config for Route53", "region", exporterConfig.Route53Config.Region, "err", err)
			return collectors, err
		}
		r53Exporter := pkg.NewRoute53Exporter(regionCfg, logger, exporterConfig.Route53Config, awsAccountId)
		collectors = append(collectors, r53Exporter)
		go r53Exporter.CollectLoop()
	}
	logger.Info("Will ElastiCache metrics be gathered?", "elasticache-enabled", exporterConfig.ElastiCacheConfig.Enabled)
	var elasticacheConfigs []aws.Config
	if exporterConfig.ElastiCacheConfig.Enabled {
		for _, region := range exporterConfig.ElastiCacheConfig.Regions {
			regionCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
			if err != nil {
				logger.Error("Could not load AWS config for ElastiCache", "region", region, "err", err)
				return collectors, err
			}
			elasticacheConfigs = append(elasticacheConfigs, regionCfg)
		}
		elasticacheExporter := pkg.NewElastiCacheExporter(elasticacheConfigs, logger, exporterConfig.ElastiCacheConfig, awsAccountId)
		collectors = append(collectors, elasticacheExporter)
		go elasticacheExporter.CollectLoop()
	}
	logger.Info("Will MSK metrics be gathered?", "msk-enabled", exporterConfig.MskConfig.Enabled)
	var mskConfigs []aws.Config
	if exporterConfig.MskConfig.Enabled {
		for _, region := range exporterConfig.MskConfig.Regions {
			regionCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
			if err != nil {
				logger.Error("Could not load AWS config for MSK", "region", region, "err", err)
				return collectors, err
			}
			mskConfigs = append(mskConfigs, regionCfg)
		}
		mskExporter := pkg.NewMSKExporter(mskConfigs, logger, exporterConfig.MskConfig, awsAccountId)
		collectors = append(collectors, mskExporter)
		go mskExporter.CollectLoop()
	}
	logger.Info("Will IAM metrics be gathered?", "iam-enabled", exporterConfig.IamConfig.Enabled)
	if exporterConfig.IamConfig.Enabled {
		// IAM is global, this region just for AWS SDK initialization
		regionCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(exporterConfig.IamConfig.Region))
		if err != nil {
			logger.Error("Could not load AWS config for IAM", "region", exporterConfig.IamConfig.Region, "err", err)
			return collectors, err
		}
		iamExporter := pkg.NewIAMExporter(regionCfg, logger, exporterConfig.IamConfig, awsAccountId)
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
