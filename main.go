package main

import (
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/prometheus/common/log"

	"github.com/prometheus/client_golang/prometheus"
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

	config := aws.NewConfig().WithCredentials(creds).WithRegion(os.Getenv("REGION"))
	sess := session.Must(session.NewSession(config))

	exporterMetrics = NewExporterMetrics(sess)
	prometheus.MustRegister(
		exporterMetrics,
		NewRDSExporter(sess),
	)

	http.Handle(*metricsPath, prometheus.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>AWS Resources Exporter</title></head>
             <body>
             <h1>AWS Resources Exporter</h1>
             <p><a href='` + *metricsPath + `'>Metrics</a></p>
             </body>
             </html>`))
	})
	log.Infoln("Starting HTTP server on", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
