package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	PrometheusExporter = NewPrometheusExporter()
	Varnish            = &varnish{
		Version:  NewVarnishVersion(),
		Exporter: NewVarnishExporter(),
	}

	StartParams = &startParams{
		Host: "",
		Port: 9102,
		Path: "/metrics",
	}
	logger *log.Logger
)

type varnish struct {
	Version  *varnishVersion
	Exporter *varnishExporter
}

type startParams struct {
	Host    string
	Port    int
	Path    string
	Verbose bool
	Test    bool
	Raw     bool
}

func init() {
	flag.StringVar(&StartParams.Host, "host", StartParams.Host, "HTTP server host")
	flag.IntVar(&StartParams.Port, "port", StartParams.Port, "HTTP server port")
	flag.StringVar(&StartParams.Path, "path", StartParams.Path, "HTTP server path that exposes metrics")
	flag.BoolVar(&StartParams.Verbose, "verbose", StartParams.Verbose, "Verbose logging")
	flag.BoolVar(&StartParams.Test, "test", StartParams.Test, "Test varnishstat availability, prints available metrics and exits")
	flag.BoolVar(&StartParams.Raw, "raw", StartParams.Test, "Raw stdout logging without timestamps")
	flag.Parse()

	logger = log.New(os.Stdout, "", log.Ldate|log.Ltime)

	if len(StartParams.Path) == 0 || StartParams.Path[0] != '/' {
		logFatal("-path cannot be empty and must start with a slash '/', given %q", StartParams.Path)
	}
}

func main() {
	if b, err := json.MarshalIndent(StartParams, "", "  "); err == nil {
		logInfo("Starting up %s", b)
	} else {
		logFatal(err.Error())
	}

	if err := Varnish.Version.Initialize(); err != nil {
		logFatal("Varnish version initialize failed: %s", err.Error())
	}

	t := time.Now()
	if err := Varnish.Exporter.Initialize(); err != nil {
		logFatal("Varnish exporter initialize failed: %s", err.Error())
	}
	logInfo("Initialized %d metrics from %s %s in %s\n\n", len(Varnish.Exporter.metrics), varnishstatExe, Varnish.Version, time.Now().Sub(t).String())

	if err := PrometheusExporter.exposeMetrics(Varnish.Exporter.metrics); err != nil {
		logFatal("Exposing metrics failed: %s", err.Error())
	}

	if StartParams.Test {
		dumpMetrics(PrometheusExporter)

		t = time.Now()
		if errUpdate := Varnish.Exporter.Update(); errUpdate == nil {
			logInfo("Executed values update in %s", time.Now().Sub(t))
		} else {
			logFatal("VarnishExporter.Update: %s", errUpdate.Error())
		}
	}

	prometheus.MustRegister(PrometheusExporter)

	if StartParams.Test {
		os.Exit(0)
	}

	// Start serving
	listenAddress := fmt.Sprintf("%s:%d", StartParams.Host, StartParams.Port)
	logInfo("Server starting on %s", listenAddress)

	http.Handle(StartParams.Path, prometheus.Handler())
	logFatalError(http.ListenAndServe(listenAddress, nil))
}
