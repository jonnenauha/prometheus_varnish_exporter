package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	PrometheusExporter = NewPrometheusExporter()
	VarnishVersion     = NewVarnishVersion()

	StartParams = &startParams{
		Host:   "",
		Port:   9102,
		Path:   "/metrics",
		Params: &varnishstatParams{},
	}
	logger *log.Logger
)

type startParams struct {
	Host    string
	Port    int
	Path    string
	Params  *varnishstatParams
	Verbose bool
	Test    bool
	Raw     bool
}

type varnishstatParams struct {
	Instance string
	VSM      string
}

func (p *varnishstatParams) isEmpty() bool {
	return p.Instance == "" && p.VSM == ""
}

func (p *varnishstatParams) make() (params []string) {
	// -n
	if p.Instance != "" {
		params = append(params, "-n", p.Instance)
	}
	// -N is not supported by 3.x
	if p.VSM != "" && VarnishVersion != nil && VarnishVersion.Major >= 4 {
		params = append(params, "-N", p.VSM)
	}
	return params
}

func init() {
	flag.StringVar(&StartParams.Host, "host", StartParams.Host, "HTTP server host")
	flag.IntVar(&StartParams.Port, "port", StartParams.Port, "HTTP server port")
	flag.StringVar(&StartParams.Path, "path", StartParams.Path, "HTTP server path that exposes metrics")
	flag.StringVar(&StartParams.Params.Instance, "n", StartParams.Params.VSM, "varnishstat -n value")
	flag.StringVar(&StartParams.Params.VSM, "N", StartParams.Params.VSM, "varnishstat -N value")
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

	// Initialize
	if err := VarnishVersion.Initialize(); err != nil {
		logFatal("Varnish version initialize failed: %s", err.Error())
	}
	if err := PrometheusExporter.Initialize(); err != nil {
		logFatal("Prometheus exporter initialize failed: %s", err.Error())
	}

	// Test mode
	if StartParams.Test {
		metrics := make(chan prometheus.Metric)
		go func() {
			for m := range metrics {
				logInfo("%s", m.Desc())
			}
		}()
		logFatalError(scrapeVarnish(metrics))
		close(metrics)
		os.Exit(0)
	}

	// Start serving
	listenAddress := fmt.Sprintf("%s:%d", StartParams.Host, StartParams.Port)
	logInfo("Server starting on %s", listenAddress)

	prometheus.MustRegister(PrometheusExporter)

	http.Handle(StartParams.Path, prometheus.Handler())
	logFatalError(http.ListenAndServe(listenAddress, nil))
}
