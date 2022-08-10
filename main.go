package main

import (
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/yaml.v2"
)

const (
	namespace                      = "aws_resources_exporter"
	DEFAULT_TIMEOUT  time.Duration = 30 * time.Second
	CONFIG_FILE_PATH               = "./aws-resource-exporter-config.yaml"
)

var (
	listenAddress = kingpin.Flag("web.listen-address", "The address to listen on for HTTP requests.").Default(":9115").String()
	metricsPath   = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()

	exporterMetrics *ExporterMetrics
)

func main() {
	os.Exit(run())
}

type BaseConfig struct {
	Enabled bool     `yaml:"enabled"`
	Regions []string `yaml:"regions"`
}

type VPCConfig struct {
	BaseConfig `yaml:"base,inline"`
	Timeout    time.Duration `yaml:"timeout"`
}

type Route53Config struct {
	BaseConfig `yaml:"base,inline"`
	Timeout    time.Duration `yaml:"timeout"`
}

type Config struct {
	RdsConfig     BaseConfig    `yaml:"rds"`
	VpcConfig     VPCConfig     `yaml:"vpc"`
	Route53Config Route53Config `yaml:"route53"`
}

func loadExporterConfiguration(logger log.Logger, configFile string) (*Config, error) {
	var config Config
	file, err := ioutil.ReadFile(configFile)
	if err != nil {
		level.Error(logger).Log("Could not load configuration file")
		return nil, errors.New("Could not load configuration file: " + configFile)
	}
	yaml.Unmarshal(file, &config)
	return &config, nil
}

func setupCollectors(logger log.Logger, configFile string, creds *credentials.Credentials) ([]prometheus.Collector, error) {
	var collectors []prometheus.Collector
	config, err := loadExporterConfiguration(logger, configFile)
	if err != nil {
		return nil, err
	}
	level.Info(logger).Log("msg", "Configuring vpc with regions", "regions", strings.Join(config.VpcConfig.Regions, ","))
	level.Info(logger).Log("msg", "Configuring rds with regions", "regions", strings.Join(config.RdsConfig.Regions, ","))
	level.Info(logger).Log("msg", "Configuring route53 with regions", "regions", strings.Join(config.Route53Config.Regions, ","))
	var vpcSessions []*session.Session
	level.Info(logger).Log("msg", "Will VPC metrics be gathered?", "vpc-enabled", config.VpcConfig.Enabled)
	if config.VpcConfig.Enabled {
		for _, region := range config.VpcConfig.Regions {
			config := aws.NewConfig().WithCredentials(creds).WithRegion(region)
			sess := session.Must(session.NewSession(config))
			vpcSessions = append(vpcSessions, sess)
		}
		collectors = append(collectors, NewVPCExporter(vpcSessions, logger, config.VpcConfig.Timeout))
	}
	level.Info(logger).Log("msg", "Will RDS metrics be gathered?", "rds-enabled", config.RdsConfig.Enabled)
	var rdsSessions []*session.Session
	if config.RdsConfig.Enabled {
		for _, region := range config.RdsConfig.Regions {
			config := aws.NewConfig().WithCredentials(creds).WithRegion(region)
			sess := session.Must(session.NewSession(config))
			rdsSessions = append(rdsSessions, sess)
		}
		collectors = append(collectors, NewRDSExporter(rdsSessions, logger))
	}
	level.Info(logger).Log("msg", "Will Route53 metrics be gathered?", "route53-enabled", config.Route53Config.Enabled)
	var route53Sessions []*session.Session
	if config.Route53Config.Enabled {
		for _, region := range config.Route53Config.Regions {
			config := aws.NewConfig().WithCredentials(creds).WithRegion(region)
			sess := session.Must(session.NewSession(config))
			route53Sessions = append(route53Sessions, sess)
		}
		collectors = append(collectors, NewRoute53Exporter(route53Sessions[0], logger, config.Route53Config.Timeout))
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

	creds := credentials.NewEnvCredentials()
	if _, err := creds.Get(); err != nil {
		level.Error(logger).Log("msg", "Could not get AWS credentials from env variables", "err", err)
		return 1
	}

	exporterMetrics = NewExporterMetrics()
	var configFile string
	if path := os.Getenv("AWS_RESOURCE_EXPORTER_CONFIG_FILE"); path != "" {
		configFile = path
	} else {
		configFile = CONFIG_FILE_PATH
	}
	cs, err := setupCollectors(logger, configFile, creds)
	if err != nil {
		level.Error(logger).Log("msg", "Could not load configuration file", "err", err)
		return 1
	}
	collectors := append(cs, exporterMetrics)
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
