package main

import (
	"errors"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
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
	namespace                     = "aws_resources_exporter"
	DEFAULT_TIMEOUT time.Duration = 10 * time.Second
	FALLBACK_REGION               = "us-east-1"
)

var (
	listenAddress = kingpin.Flag("web.listen-address", "The address to listen on for HTTP requests.").Default(":9115").String()
	metricsPath   = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()

	exporterMetrics *ExporterMetrics
)

func main() {
	os.Exit(run())
}

// GetRegions can retrieve 1, N or all regions.
// 1 region can be passed using AWS_REGIONS or AWS_REGION
// N regions can be passed using AWS_REGIONS
// ALL regions can be passed using AWS_REGIONS (via 'all')
// If both AWS_REGIONS and AWS_REGION is set the older variable (AWS_REGION) will take precendence to not break old use cases.
func getRegions(logger log.Logger, creds *credentials.Credentials) ([]string, error) {
	// Handle the case if multiple regions are supposed to be used.
	var useRegions []string
	// The normal usage of a single region being monitored
	awsRegion := os.Getenv("AWS_REGION")
	if awsRegion == "" {
		level.Info(logger).Log("msg", "AWS_REGION undefined.")
	} else {
		return []string{awsRegion}, nil
	}

	awsRegions := os.Getenv("AWS_REGIONS")
	switch strings.ToLower(awsRegions) {
	case "":
		level.Info(logger).Log("msg", "AWS_REGIONS undefined, won't run for multiple regions")
	case "all":
		config := aws.NewConfig().WithCredentials(creds).WithRegion(FALLBACK_REGION)
		sess := session.Must(session.NewSession(config))
		ec2svc := ec2.New(sess)
		allRegions, err := ec2svc.DescribeRegions(&ec2.DescribeRegionsInput{})
		if err != nil {
			level.Error(logger).Log("msg", "Could not retrieve all regions from account", "err", err)
			return nil, err
		}
		for _, region := range allRegions.Regions {
			useRegions = append(useRegions, *region.RegionName)
		}
	default:
		useRegions = strings.Split(awsRegions, ",")
	}

	if awsRegion == "" && awsRegions == "" {
		level.Error(logger).Log("msg", "AWS_REGION or AWS_REGIONS has to be defined")
		return nil, errors.New("AWS_REGION or AWS_REGIONS must be defined")
	}

	return useRegions, nil
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

	var timeout time.Duration
	envTimeout := os.Getenv("AWS_RESOURCE_EXPORTER_TIMEOUT")
	if envTimeout != "" {
		parsedTimeout, err := time.ParseDuration(envTimeout)
		if err != nil {
			level.Error(logger).Log("msg", "Could not parse timeout duration passed via 'AWS_RESOURCE_EXPORTER_TIMEOUT'", "err", err)
			timeout = DEFAULT_TIMEOUT
		} else {
			timeout = parsedTimeout
		}
	} else {
		timeout = DEFAULT_TIMEOUT
	}

	creds := credentials.NewEnvCredentials()
	if _, err := creds.Get(); err != nil {
		level.Error(logger).Log("msg", "Could not get AWS credentials from env variables", "err", err)
		return 1
	}

	regions, err := getRegions(logger, creds)
	if err != nil {
		level.Error(logger).Log("msg", "Region configuration invalid", "err", err)
		return 1
	}

	exporterMetrics = NewExporterMetrics()
	switch len(regions) {
	case 1:
		level.Info(logger).Log("msg", "Initializing RDS and VPC exporter for region", "region", regions[0])
		config := aws.NewConfig().WithCredentials(creds).WithRegion(regions[0])
		sess := session.Must(session.NewSession(config))
		prometheus.MustRegister(exporterMetrics, NewRDSExporter(sess, logger), NewVPCExporter([]*session.Session{sess}, logger, timeout))
	default:
		level.Info(logger).Log("msg", "Initializing VPC exporter for multiple regions")
		var sessions []*session.Session
		for _, awsRegion := range regions {
			level.Info(logger).Log("msg", "Initializing session for region:", "region", awsRegion)
			config := aws.NewConfig().WithCredentials(creds).WithRegion(awsRegion)
			sess := session.Must(session.NewSession(config))
			sessions = append(sessions, sess)
		}
		var collectors []prometheus.Collector
		collectors = append(collectors, exporterMetrics, NewVPCExporter(sessions, logger, timeout))
		prometheus.MustRegister(
			collectors...,
		)
	}

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
