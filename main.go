package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	PrometheusExporter = NewPrometheusExporter()
	VarnishVersion     = NewVarnishVersion()

	StartParams = &startParams{
		ListenAddress: ":9131", // Reserved and publicly announced at https://github.com/prometheus/prometheus/wiki/Default-port-allocations
		Path:          "/metrics",
		Params:        &varnishstatParams{},
	}
	logger *log.Logger
)

type startParams struct {
	ListenAddress string
	Path          string
	Params        *varnishstatParams
	Verbose       bool
	Test          bool
	Raw           bool
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
	// prometheus conventions
	flag.StringVar(&StartParams.ListenAddress, "web.listen-address", StartParams.ListenAddress, "Address on which to expose metrics and web interface.")
	flag.StringVar(&StartParams.Path, "web.telemetry-path", StartParams.Path, "Path under which to expose metrics.")

	// varnish
	flag.StringVar(&StartParams.Params.Instance, "n", StartParams.Params.Instance, "varnishstat -n value.")
	flag.StringVar(&StartParams.Params.VSM, "N", StartParams.Params.VSM, "varnishstat -N value.")

	// modes
	flag.BoolVar(&StartParams.Verbose, "verbose", StartParams.Verbose, "Verbose logging.")
	flag.BoolVar(&StartParams.Test, "test", StartParams.Test, "Test varnishstat availability, prints available metrics and exits.")
	flag.BoolVar(&StartParams.Raw, "raw", StartParams.Test, "Raw stdout logging without timestamps.")

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
	logInfo("Server starting on %s with metrics path %s", StartParams.ListenAddress, StartParams.Path)

	prometheus.MustRegister(PrometheusExporter)

	// 400 Bad Request for anything except the configured metrics path.
	// If you want to make the path obscure to hide it from snooping while still exposing
	// it to the public web, you don't want to show some "go here instead" message at root etc.
	if StartParams.Path != "/" {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		})
	}
	// metrics
	http.Handle(StartParams.Path, prometheus.Handler())
	logFatalError(http.ListenAndServe(StartParams.ListenAddress, nil))
}
