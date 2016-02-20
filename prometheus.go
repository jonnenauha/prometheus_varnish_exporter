package main

import (
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	exporterNamespace = "varnish"
)

// prometheusExporter

type prometheusExporter struct {
	sync.RWMutex

	up      prometheus.Gauge
	version prometheus.Gauge
}

func NewPrometheusExporter() *prometheusExporter {
	return &prometheusExporter{
		// @todo varnishstat never fails, even if varnish is not running.
		// Figure our a reliable way to detect if varnish is running and use that for up.
		up: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: exporterNamespace,
			Name:      "up",
			Help:      "Was the last scrape of varnish successful.",
		}),
	}
}

func (pe *prometheusExporter) Initialize() error {
	pe.version = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   exporterNamespace,
		Name:        "version",
		Help:        "Varnish version information",
		ConstLabels: VarnishVersion.Labels(),
	})
	pe.version.Set(1)
	return nil
}

// Implements prometheus.Collector
func (pe *prometheusExporter) Describe(ch chan<- *prometheus.Desc) {
	start := time.Now()

	ch <- pe.up.Desc()
	ch <- pe.version.Desc()

	if StartParams.Verbose {
		logInfo("prometheus.Collector.Describe  %s", time.Now().Sub(start))
	}
}

// Implements prometheus.Collector
func (pe *prometheusExporter) Collect(ch chan<- prometheus.Metric) {
	start := time.Now()

	pe.Lock()
	defer pe.Unlock()

	if err := scrapeVarnish(ch); err == nil {
		pe.up.Set(1)
	} else {
		logError(err.Error())
		pe.up.Set(0)
	}

	ch <- pe.up
	ch <- pe.version

	if StartParams.Verbose {
		logInfo("prometheus.Collector.Collect   %s", time.Now().Sub(start))
	}
}

// utils

type group struct {
	name     string
	prefixes []string
}

var (
	groups = []group{
		// @note varnish 3.x does not seem to mark 'MAIN.' prefixes
		group{name: "backend", prefixes: []string{
			"vbe.",
			// varnish 4.x
			"main.backend_",
			// varnish 3.x
			"backend_",
		}},
		group{name: "mempool", prefixes: []string{
			"mempool.",
		}},
		group{name: "lck", prefixes: []string{
			"lck.",
		}},
		group{name: "sma", prefixes: []string{
			"sma.",
		}},
		group{name: "mgt", prefixes: []string{
			"mgt.",
		}},
		// must be last so above groups have a opportunity to override
		group{name: "main", prefixes: []string{
			"main.",
		}},
	}
)

const (
	fq_main_fetch       = exporterNamespace + "_main_fetch"
	fq_main_fetch_total = exporterNamespace + "_main_s_fetch"
)

// https://prometheus.io/docs/practices/naming/
func computePrometheusInfo(vName, vGroup, vIdentifier, vDescription string) (name, description string, labelKeys, labelValues []string) {
	// name and description
	{
		fq := strings.ToLower(vName)
		// Remove unique identifiers from name to group similar metrics by labeling
		if len(vIdentifier) > 0 {
			fq = strings.Replace(fq, "."+strings.ToLower(vIdentifier), "", -1)
		}
		// Make sure our group is prefixed only once
		fq = prometheusTrimGroupPrefix(fq)
		// Build fq name
		name = exporterNamespace + "_" + vGroup + "_" + strings.Replace(fq, ".", "_", -1)
		description = vDescription
	}
	// labels: can alter final name and description
	{
		if len(vIdentifier) > 0 {
			if isVBE := startsWith(vName, "VBE.", caseSensitive); isVBE {
				// @todo this is quick and dirty, do regexp?
				if VarnishVersion.Major == 4 {
					// <uuid>.<name>
					if len(vIdentifier) > 37 && vIdentifier[8] == '-' && vIdentifier[36] == '.' {
						labelKeys, labelValues = append(labelKeys, "id"), append(labelValues, vIdentifier[0:36])
						labelKeys, labelValues = append(labelKeys, "backend"), append(labelValues, vIdentifier[37:])
					}
				} else if VarnishVersion.Major == 3 {
					// <name>(<ip>,<something>,<port>)
					iStart, iEnd := strings.Index(vIdentifier, "("), strings.Index(vIdentifier, ")")
					if iStart > 0 && iEnd > 1 && iStart < iEnd {
						labelKeys, labelValues = append(labelKeys, "id"), append(labelValues, vIdentifier[iStart+1:iEnd])
						labelKeys, labelValues = append(labelKeys, "backend"), append(labelValues, vIdentifier[0:iStart])
					}
				}
			}
			if len(labelKeys) == 0 {
				labelKeys, labelValues = append(labelKeys, "id"), append(labelValues, vIdentifier)
			}
		}
		// name grouping bt moving part of the fq name as a label
		if name == fq_main_fetch_total {
			labelKeys, labelValues = append(labelKeys, "type"), append(labelValues, "total")
			name, description = fq_main_fetch, "Number of fetches"
		} else if startsWith(name, fq_main_fetch+"_", caseSensitive) {
			// If name is manipulated to be the same for multiple metrics
			// the description needs to match as well.
			labelKeys, labelValues = append(labelKeys, "type"), append(labelValues, name[len(fq_main_fetch)+1:])
			name, description = fq_main_fetch, "Number of fetches"
		}

	}
	return name, description, labelKeys, labelValues
}

func prometheusTrimGroupPrefix(name string) string {
	nameLower := strings.ToLower(name)
	for _, group := range groups {
		for _, prefix := range group.prefixes {
			if startsWith(nameLower, prefix, caseSensitive) {
				return name[len(prefix):]
			}
		}
	}
	return name
}

// Always returns at least one main label
func prometheusGroup(vName string) string {
	vNameLower := strings.ToLower(vName)
	for _, group := range groups {
		if startsWithAny(vNameLower, group.prefixes, caseSensitive) {
			return group.name
		}
	}
	return "main"
}
