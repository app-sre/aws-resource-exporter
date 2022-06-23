package main

import (
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	namespace                     = "aws_resources_exporter"
	DEFAULT_TIMEOUT time.Duration = 10 * time.Second
)

var (
	listenAddress = kingpin.Flag("web.listen-address", "The address to listen on for HTTP requests.").Default(":9115").String()
	metricsPath   = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()

	exporterMetrics *ExporterMetrics
)

func main() {
	os.Exit(run())
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

	awsRegion := os.Getenv("AWS_REGION")
	if awsRegion == "" {
		level.Error(logger).Log("msg", "AWS_REGION has to be defined")
		return 1
	}
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

	config := aws.NewConfig().WithCredentials(creds).WithRegion(awsRegion)
	sess := session.Must(session.NewSession(config))

	exporterMetrics = NewExporterMetrics(sess)
	prometheus.MustRegister(
		exporterMetrics,
		NewRDSExporter(sess, logger),
		NewVPCExporter(sess, logger, timeout),
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
