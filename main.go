package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"
)

var (
	StartParams = &startParams{
		Host: "0.0.0.0",
		Port: 9102,
	}
	logger *log.Logger
)

type startParams struct {
	Host string
	Port int
	Test bool
	Raw  bool
}

func init() {
	flag.StringVar(&StartParams.Host, "host", StartParams.Host, "HTTP server host")
	flag.IntVar(&StartParams.Port, "port", StartParams.Port, "HTTP server port")
	flag.BoolVar(&StartParams.Test, "test", StartParams.Test, "Test varnishstat availability, prints available metrics and exits")
	flag.BoolVar(&StartParams.Raw, "raw", StartParams.Test, "Raw stdout logging, no timestamps")
	flag.Parse()

	logger = log.New(os.Stdout, "", log.Ldate|log.Ltime)
}

func main() {
	if b, err := json.MarshalIndent(StartParams, "", "  "); err == nil {
		logInfo("Starting up %s", b)
	} else {
		logFatal(err.Error())
	}

	if err := VarnishExporter.queryVersion(); err != nil {
		logFatal("Querying version failed: %s", err.Error())
	}

	t := time.Now()
	if err := VarnishExporter.queryMetrics(); err != nil {
		logFatal("Querying metrics failed: %s", err.Error())
	}
	logInfo("Queried %d metrics from %s %s in %s\n\n", len(VarnishExporter.metrics), varnishstatExe, VarnishExporter.version, time.Now().Sub(t).String())

	if err := PrometheusExporter.exposeMetrics(VarnishExporter.metrics); err != nil {
		logFatal("Exposing metrics failed: %s", err.Error())
	}

	if StartParams.Test {
		// pretty print based on group
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
		os.Exit(1)
	}
}

func logRaw(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

func logTitle(format string, args ...interface{}) {
	logInfo(format, args...)

	title := strings.Repeat("-", len(fmt.Sprintf(format, args...)))
	if len(title) > 0 {
		logInfo(title)
	}
}

func logInfo(format string, args ...interface{}) {
	if StartParams.Raw {
		logRaw(format, args...)
	} else {
		logger.Printf(format, args...)
	}
}

func logWarn(format string, args ...interface{}) {
	format = "[WARN] " + format
	if StartParams.Raw {
		logRaw(format, args...)
	} else {
		logger.Printf(format, args...)
	}
}

func logError(format string, args ...interface{}) {
	format = "[ERROR] " + format
	if StartParams.Raw {
		logRaw(format, args...)
	} else {
		logger.Printf(format, args...)
	}
}

func logFatal(format string, args ...interface{}) {
	format = "[FATAL] " + format
	if StartParams.Raw {
		logRaw(format, args...)
	} else {
		logger.Printf(format, args...)
	}
	os.Exit(1)
}
