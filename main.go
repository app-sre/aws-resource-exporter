package main

import (
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/prometheus/common/log"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	namespace = "aws_resources_exporter"
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
	awsRegion := os.Getenv("AWS_REGION")
	if awsRegion == "" {
		log.Fatalln("AWS_REGION has to be defined")
	}

	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print(namespace))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	log.Infoln("Starting", namespace, version.Info())
	log.Infoln("Build context", version.BuildContext())

	creds := credentials.NewEnvCredentials()
	if _, err := creds.Get(); err != nil {
		log.Fatalln(err)
	}

	config := aws.NewConfig().WithCredentials(creds).WithRegion(awsRegion)
	sess := session.Must(session.NewSession(config))

	exporterMetrics = NewExporterMetrics(sess)
	prometheus.MustRegister(
		exporterMetrics,
		NewRDSExporter(sess),
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
		log.Infoln("Starting HTTP server on", *listenAddress)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Errorf("Error starting HTTP server: %v", err)
			close(srvc)
		}
	}()

	for {
		select {
		case <-term:
			log.Infoln("Received SIGTERM, exiting gracefully...")
			return 0
		case <-srvc:
			return 1
		}
	}
}
