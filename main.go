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

type DefaultConfig struct {
	Regions string        `yaml:"regions"`
	Timeout time.Duration `yaml:"timeout"`
}

type Config struct {
	DefaultConfig DefaultConfig `yaml:"default"`
	RdsConfig     DefaultConfig `yaml:"rds"`
	VpcConfig     DefaultConfig `yaml:"vpc"`
	Route53Config DefaultConfig `yaml:"route53"`
}

func (c *DefaultConfig) TimeoutWithFallBack(config *Config) time.Duration {
	if c.Timeout == 0*time.Second {
		return config.DefaultConfig.Timeout
	}
	return c.Timeout
}

func (c *DefaultConfig) RegionsWithFallback(config *Config) []string {
	regions, err := c.ParseRegions()
	if len(regions) == 0 || err != nil {
		if regions, err = config.DefaultConfig.ParseRegions(); err != nil {
			return []string{os.Getenv("AWS_REGION")}
		} else {
			return regions
		}
	}
	return regions
}

func (c *DefaultConfig) ParseRegions() ([]string, error) {
	switch strings.ToLower(c.Regions) {
	case "":
		return []string{}, nil
	default:
		return strings.Split(c.Regions, ","), nil
	}
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
	vpcRegions := config.VpcConfig.RegionsWithFallback(config)
	level.Info(logger).Log("msg", "Configuring vpc with regions", "regions", strings.Join(vpcRegions, ","))
	rdsRegions := config.RdsConfig.RegionsWithFallback(config)
	level.Info(logger).Log("msg", "Configuring rds with regions", "regions", strings.Join(rdsRegions, ","))
	route53Regions := config.Route53Config.RegionsWithFallback(config)
	level.Info(logger).Log("msg", "Configuring route53 with regions", "regions", strings.Join(route53Regions, ","))
	var vpcSessions []*session.Session
	for _, region := range vpcRegions {
		config := aws.NewConfig().WithCredentials(creds).WithRegion(region)
		sess := session.Must(session.NewSession(config))
		vpcSessions = append(vpcSessions, sess)
	}
	var rdsSessions []*session.Session
	for _, region := range rdsRegions {
		config := aws.NewConfig().WithCredentials(creds).WithRegion(region)
		sess := session.Must(session.NewSession(config))
		rdsSessions = append(rdsSessions, sess)
	}
	var route53Sessions []*session.Session
	for _, region := range route53Regions {
		config := aws.NewConfig().WithCredentials(creds).WithRegion(region)
		sess := session.Must(session.NewSession(config))
		route53Sessions = append(route53Sessions, sess)
	}
	collectors = append(collectors, NewVPCExporter(vpcSessions, logger, config.VpcConfig.TimeoutWithFallBack(config)))
	collectors = append(collectors, NewRDSExporter(rdsSessions, logger))
	collectors = append(collectors, NewRoute53Exporter(route53Sessions[0], logger, config.Route53Config.TimeoutWithFallBack(config)))

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
