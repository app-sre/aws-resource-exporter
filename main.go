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

func getAwsAccountNumber(ctx context.Context, logger *slog.Logger, cfg aws.Config) (string, error) {
	stsClient := sts.NewFromConfig(cfg)
	identityOutput, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		logger.Error("Could not retrieve caller identity of the aws account", "err", err)
		return "", err
	}
	return *identityOutput.Account, nil
}

func setupCollectors(logger *slog.Logger, configFile string) ([]prometheus.Collector, error) {
	var collectors []prometheus.Collector
	cfg, err := pkg.LoadExporterConfiguration(logger, configFile)
	if err != nil {
		return nil, err
	}
	logger.Info("Configuring vpc with regions", "regions", strings.Join(cfg.VpcConfig.Regions, ","))
	logger.Info("Configuring rds with regions", "regions", strings.Join(cfg.RdsConfig.Regions, ","))
	logger.Info("Configuring ec2 with regions", "regions", strings.Join(cfg.EC2Config.Regions, ","))
	logger.Info("Configuring route53 with region", "region", cfg.Route53Config.Region)
	logger.Info("Configuring elasticache with regions", "regions", strings.Join(cfg.ElastiCacheConfig.Regions, ","))
	logger.Info("Configuring msk with regions", "regions", strings.Join(cfg.MskConfig.Regions, ","))
	logger.Info("Will VPC metrics be gathered?", "vpc-enabled", cfg.VpcConfig.Enabled)
	logger.Info("Will IAM metrics be gathered?", "iam-enabled", cfg.IamConfig.Enabled)

	sessionRegion := "us-east-1"
	if sr := os.Getenv("AWS_REGION"); sr != "" {
		sessionRegion = sr
	}

	ctx := context.TODO()

	// Create a single session here, because we need the accountid, before we create the other configs
	awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(sessionRegion))
	if err != nil {
		return nil, err
	}

	awsAccountId, err := getAwsAccountNumber(ctx, logger, awsCfg)
	if err != nil {
		return collectors, err
	}
	var vpcCfgs []aws.Config
	if cfg.VpcConfig.Enabled {
		for _, region := range cfg.VpcConfig.Regions {
			awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
			if err != nil {
				return nil, err
			}
			vpcCfgs = append(vpcCfgs, awsCfg)
		}
		vpcExporter := pkg.NewVPCExporter(vpcCfgs, logger, cfg.VpcConfig, awsAccountId)
		collectors = append(collectors, vpcExporter)
		go vpcExporter.CollectLoop()
	}
	logger.Info("Will RDS metrics be gathered?", "rds-enabled", cfg.RdsConfig.Enabled)
	var rdsCfgs []aws.Config
	if cfg.RdsConfig.Enabled {
		for _, region := range cfg.RdsConfig.Regions {
			awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
			if err != nil {
				return nil, err
			}
			rdsCfgs = append(rdsCfgs, awsCfg)
		}
		rdsExporter := pkg.NewRDSExporter(rdsCfgs, logger, cfg.RdsConfig, awsAccountId)
		collectors = append(collectors, rdsExporter)
		go rdsExporter.CollectLoop()
	}
	logger.Info("Will EC2 metrics be gathered?", "ec2-enabled", cfg.EC2Config.Enabled)
	var ec2Cfgs []aws.Config
	if cfg.EC2Config.Enabled {
		for _, region := range cfg.EC2Config.Regions {
			awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
			if err != nil {
				return nil, err
			}
			ec2Cfgs = append(ec2Cfgs, awsCfg)
		}
		ec2Exporter := pkg.NewEC2Exporter(ec2Cfgs, logger, cfg.EC2Config, awsAccountId)
		collectors = append(collectors, ec2Exporter)
		go ec2Exporter.CollectLoop()
	}
	logger.Info("Will Route53 metrics be gathered?", "route53-enabled", cfg.Route53Config.Enabled)
	if cfg.Route53Config.Enabled {
		awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(cfg.Route53Config.Region))
		if err != nil {
			return nil, err
		}
		r53Exporter := pkg.NewRoute53Exporter(awsCfg, logger, cfg.Route53Config, awsAccountId)
		collectors = append(collectors, r53Exporter)
		go r53Exporter.CollectLoop()
	}
	logger.Info("Will ElastiCache metrics be gathered?", "elasticache-enabled", cfg.ElastiCacheConfig.Enabled)
	var elasticacheCfgs []aws.Config
	if cfg.ElastiCacheConfig.Enabled {
		for _, region := range cfg.ElastiCacheConfig.Regions {
			awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
			if err != nil {
				return nil, err
			}
			elasticacheCfgs = append(elasticacheCfgs, awsCfg)
		}
		elasticacheExporter := pkg.NewElastiCacheExporter(elasticacheCfgs, logger, cfg.ElastiCacheConfig, awsAccountId)
		collectors = append(collectors, elasticacheExporter)
		go elasticacheExporter.CollectLoop()
	}
	logger.Info("Will MSK metrics be gathered?", "msk-enabled", cfg.MskConfig.Enabled)
	var mskCfgs []aws.Config
	if cfg.MskConfig.Enabled {
		for _, region := range cfg.MskConfig.Regions {
			awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
			if err != nil {
				return nil, err
			}
			mskCfgs = append(mskCfgs, awsCfg)
		}
		mskExporter := pkg.NewMSKExporter(mskCfgs, logger, cfg.MskConfig, awsAccountId)
		collectors = append(collectors, mskExporter)
		go mskExporter.CollectLoop()
	}
	logger.Info("Will IAM metrics be gathered?", "iam-enabled", cfg.IamConfig.Enabled)
	if cfg.IamConfig.Enabled {
		awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(cfg.IamConfig.Region))
		if err != nil {
			return nil, err
		}
		iamExporter := pkg.NewIAMExporter(awsCfg, logger, cfg.IamConfig, awsAccountId)
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
