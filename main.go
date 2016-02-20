package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	PrometheusExporter = NewPrometheusExporter()
	VarnishExporter    = NewVarnishExporter()

	StartParams = &startParams{
		Host: "0.0.0.0",
		Port: 9102,
		Path: "/metrics",
	}
	logger *log.Logger
)

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

	t := time.Now()
	if err := VarnishExporter.Initialize(); err != nil {
		logFatal("VarnishExporter initialize failed: %s", err.Error())
	}
	logInfo("Initialized %d metrics from %s %s in %s\n\n", len(VarnishExporter.metrics), varnishstatExe, VarnishExporter.version, time.Now().Sub(t).String())

	if err := PrometheusExporter.exposeMetrics(VarnishExporter.metrics); err != nil {
		logFatal("Exposing metrics failed: %s", err.Error())
	}

	if StartParams.Test {
		metricsByGroup := make(PrometheusMetricsbyGroup, len(VarnishExporter.metrics))
		for i, m := range PrometheusExporter.metrics {
			metricsByGroup[i] = m
		}
		sort.Sort(metricsByGroup)

		logTitle("%-23s %-8s %-23s %-15s %-10s   %-11s", "Varnish Name", "Group", "Name", "Labels", "Value", "Description")
		for _, m := range metricsByGroup {
			vName := m.NameVarnish
			vSplit := 22
			if len(m.NameVarnish) > vSplit {
				if idx := strings.Index(vName, "."); idx != -1 && idx+1 < vSplit {
					vSplit = idx + 1
				}
				vName = vName[0:vSplit]
			}
			logInfo("%-23s %-8s %-23s %-15s %10.0f   %s",
				vName,
				m.Group,
				m.Name,
				m.Labels(),
				m.Value,
				m.Description,
			)
			if len(m.NameVarnish) > vSplit {
				logInfo(" %s", m.NameVarnish[vSplit:])
			}
		}

		t = time.Now()
		if errUpdate := VarnishExporter.Update(); errUpdate == nil {
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
