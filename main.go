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
	ApplicationName = "prometheus_varnish_exporter"
	Version         string
	VersionHash     string
	VersionDate     string

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
	HealthPath    string
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
	flag.StringVar(&StartParams.HealthPath, "web.health-path", StartParams.HealthPath, "Path under which to expose healthcheck. Disabled unless configured.")

	// varnish
	flag.StringVar(&StartParams.Params.Instance, "n", StartParams.Params.Instance, "varnishstat -n value.")
	flag.StringVar(&StartParams.Params.VSM, "N", StartParams.Params.VSM, "varnishstat -N value.")

	// modes
	version := false
	flag.BoolVar(&version, "version", version, "Print version and exit")
	flag.BoolVar(&StartParams.Verbose, "verbose", StartParams.Verbose, "Verbose logging.")
	flag.BoolVar(&StartParams.Test, "test", StartParams.Test, "Test varnishstat availability, prints available metrics and exits.")
	flag.BoolVar(&StartParams.Raw, "raw", StartParams.Test, "Raw stdout logging without timestamps.")

	flag.Parse()

	if version {
		fmt.Printf("%s %s\n", ApplicationName, getVersion(true))
		os.Exit(0)
	}

	logger = log.New(os.Stdout, "", log.Ldate|log.Ltime)

	if len(StartParams.Path) == 0 || StartParams.Path[0] != '/' {
		logFatal("-web.telemetry-path cannot be empty and must start with a slash '/', given %q", StartParams.Path)
	}
	if len(StartParams.HealthPath) != 0 && StartParams.HealthPath[0] != '/' {
		logFatal("-web.health-path must start with a slash '/' if configured, given %q", StartParams.HealthPath)
	}
	if StartParams.Path == StartParams.HealthPath {
		logFatal("-web.telemetry-path and -web.health-path cannot have same value")
	}
}

func main() {
	if b, err := json.MarshalIndent(StartParams, "", "  "); err == nil {
		logInfo("%s %s %s", ApplicationName, getVersion(false), b)
	} else {
		logFatal(err.Error())
	}

	// Initialize
	if err := VarnishVersion.Initialize(); err != nil {
		logFatal("Varnish version initialize failed: %s", err.Error())
	}
	logInfo("Found varnishstat %s", VarnishVersion)
	if err := PrometheusExporter.Initialize(); err != nil {
		logFatal("Prometheus exporter initialize failed: %s", err.Error())
	}

	// Test to verify everything is ok before starting the server
	{
		metrics := make(chan prometheus.Metric)
		go func() {
			for m := range metrics {
				if StartParams.Test {
					logInfo("%s", m.Desc())
				}
			}
		}()
		tStart := time.Now()
		if buf, err := scrapeVarnish(metrics); err != nil {
			logRaw("\n%s\n", buf.Bytes())
			logFatal("Test scrape failed: %s", err.Error())
		}
		close(metrics)
		logInfo("Test scrape done in %s", time.Now().Sub(tStart))
		logRaw("")
	}
	if StartParams.Test {
		return
	}

	// Start serving
	logInfo("Server starting on %s with metrics path %s", StartParams.ListenAddress, StartParams.Path)

	prometheus.MustRegister(PrometheusExporter)

	// 400 Bad Request for anything except the configured metrics path, or health if configured.
	// If you want to make the path obscure to hide it from snooping while still exposing
	// it to the public web, you don't want to show some "go here instead" message at root etc.
	if StartParams.Path != "/" {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		})
	}
	if StartParams.HealthPath != "" {
		http.HandleFunc(StartParams.HealthPath, func(w http.ResponseWriter, r *http.Request) {
			// As noted in the "up" metric, needs some way to determine if everything is actually Ok.
			// For now, this just lets us check that we're accepting connections
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, "Ok")
		})
	}
	// metrics
	http.Handle(StartParams.Path, prometheus.Handler())
	logFatalError(http.ListenAndServe(StartParams.ListenAddress, nil))
}

func getVersion(date bool) (version string) {
	if Version == "" {
		return "dev"
	}
	version = fmt.Sprintf("v%s (%s)", Version, VersionHash)
	if date {
		version += " " + VersionDate
	}
	return version
}
