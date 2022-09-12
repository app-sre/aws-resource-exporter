package main

import (
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/app-sre/aws-resource-exporter/pkg"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
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

func getAwsAccountNumber(logger log.Logger) (string, error) {
	config := aws.NewConfig().WithRegion("us-east-1")
	sess := session.Must(session.NewSession(config))
	stsClient := sts.New(sess)
	identityOutput, err := stsClient.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		level.Error(logger).Log("msg", "Could not retrieve caller identity of the aws account", "err", err)
		return "", err
	}
	return *identityOutput.Account, nil
}

func setupCollectors(logger log.Logger, configFile string) ([]prometheus.Collector, error) {
	var collectors []prometheus.Collector
	config, err := pkg.LoadExporterConfiguration(logger, configFile)
	if err != nil {
		return nil, err
	}
	level.Info(logger).Log("msg", "Configuring vpc with regions", "regions", strings.Join(config.VpcConfig.Regions, ","))
	level.Info(logger).Log("msg", "Configuring rds with regions", "regions", strings.Join(config.RdsConfig.Regions, ","))
	level.Info(logger).Log("msg", "Configuring ec2 with regions", "regions", strings.Join(config.EC2Config.Regions, ","))
	level.Info(logger).Log("msg", "Configuring route53 with region", "region", config.Route53Config.Region)
	level.Info(logger).Log("msg", "Will VPC metrics be gathered?", "vpc-enabled", config.VpcConfig.Enabled)
	awsAccountId, err := getAwsAccountNumber(logger)
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
	level.Info(logger).Log("msg", "Will RDS metrics be gathered?", "rds-enabled", config.RdsConfig.Enabled)
	var rdsSessions []*session.Session
	if config.RdsConfig.Enabled {
		for _, region := range config.RdsConfig.Regions {
			config := aws.NewConfig().WithRegion(region)
			sess := session.Must(session.NewSession(config))
			rdsSessions = append(rdsSessions, sess)
		}
		rdsExporter := pkg.NewRDSExporter(rdsSessions, logger, config.RdsConfig)
		collectors = append(collectors, rdsExporter)
		go rdsExporter.CollectLoop()
	}
	level.Info(logger).Log("msg", "Will EC2 metrics be gathered?", "ec2-enabled", config.EC2Config.Enabled)
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
	level.Info(logger).Log("msg", "Will Route53 metrics be gathered?", "route53-enabled", config.Route53Config.Enabled)
	if config.Route53Config.Enabled {
		awsConfig := aws.NewConfig().WithRegion(config.Route53Config.Region)
		sess := session.Must(session.NewSession(awsConfig))
		r53Exporter := pkg.NewRoute53Exporter(sess, logger, config.Route53Config, awsAccountId)
		collectors = append(collectors, r53Exporter)
		go r53Exporter.CollectLoop()
	}

	return collectors, nil
}

func run() int {
	promlogConfig := &promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, promlogConfig)
	kingpin.Version(version.Print(namespace))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
	logger := promlog.New(promlogConfig)

	level.Info(logger).Log("msg", "Starting"+namespace, "version", version.Info())
	level.Info(logger).Log("msg", "Build context", version.BuildContext())

	pkg.AwsExporterMetrics = pkg.NewExporterMetrics()

	var configFile string
	if path := os.Getenv("AWS_RESOURCE_EXPORTER_CONFIG_FILE"); path != "" {
		configFile = path
	} else {
		configFile = CONFIG_FILE_PATH
	}
	cs, err := setupCollectors(logger, configFile)
	if err != nil {
		level.Error(logger).Log("msg", "Could not load configuration file", "err", err)
		return 1
	}
	collectors := append(cs, pkg.AwsExporterMetrics)
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
		level.Info(logger).Log("msg", "Starting HTTP server", "address", *listenAddress)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			level.Error(logger).Log("msg", "Error starting HTTP server", "err", err)
			close(srvc)
		}
	}()

	for {
		select {
		case <-term:
			level.Info(logger).Log("msg", "Received SIGTERM, exiting gracefully...")
			return 0
		case <-srvc:
			return 1
		}
	}
}
