package main

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	PrometheusExporter = NewPrometheusExporter()
)

type PrometheusMetricsbyGroup []*prometheusMetric

func (a PrometheusMetricsbyGroup) Len() int      { return len(a) }
func (a PrometheusMetricsbyGroup) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a PrometheusMetricsbyGroup) Less(i, j int) bool {
	if a[i].Group != a[j].Group {
		return a[i].Group < a[j].Group
	}
	return a[i].Name < a[j].Name
}

// prometheusMetric

type prometheusMetric struct {
	NameVarnish string
	Name        string
	Value       float64
	Description string
	Group       string

	gauge  *prometheus.GaugeVec
	labels prometheus.Labels
}

func (p *prometheusMetric) Labels() string {
	if len(p.labels) > 0 {
		return prettyPrintsMap(p.labels)
	}
	return ""
}

// prometheusExporter

type prometheusExporter struct {
	namespace string
	metrics   []*prometheusMetric

	up                          prometheus.Gauge
	totalScrapes, failedScrapes prometheus.Counter
}

func NewPrometheusExporter() *prometheusExporter {
	namespace := "varnish"
	return &prometheusExporter{
		namespace: namespace,
		metrics:   make([]*prometheusMetric, 0),
		up: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "up",
			Help:      "Was the last scrape of varnish successful.",
		}),
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "exporter_total_scrapes",
			Help:      "Current total varnish scrapes.",
		}),
		failedScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "exporter_total_failed_scrapes",
			Help:      "Current total of varnish scrape errors",
		}),
	}
}

func (p *prometheusExporter) exposeMetrics(metrics []*varnishMetric) error {
	p.metrics = make([]*prometheusMetric, 0)

	for _, m := range metrics {
		pm := &prometheusMetric{
			NameVarnish: m.Name,
			Name:        cleanPrometheusMetricName(m),
			Value:       m.Value,
			Description: m.Description,
			Group:       prometheusGroup(m),
			labels:      prometheusExtraLabels(m),
		}
		pm.gauge = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   p.namespace,
				Name:        pm.Name,
				Help:        m.Description,
				ConstLabels: pm.labels,
			},
			[]string{pm.Group},
		)
		p.metrics = append(p.metrics, pm)
	}
	return nil
}

// https://prometheus.io/docs/practices/naming/
func cleanPrometheusMetricName(metric *varnishMetric) string {
	clean := strings.ToLower(metric.Name)
	if len(metric.Identifier) > 0 {
		clean = strings.Replace(clean, "."+strings.ToLower(metric.Identifier), "", -1)
	}
	return strings.Replace(clean, ".", "_", -1)
}

var (
	// @note varnish 3.x does not seem to mark 'MAIN.' prefixes
	backendLabelPrefixes = []string{
		"VBE.",
		// varnish 4.x
		"MAIN.backend_",
		"MAIN.n_backend",
		"MAIN.s_fetch",
		// varnish 3.x
		"backend_",
		"n_backend",
		"MAIN.s_fetch",
	}
	mempoolLabelPrefixes = []string{
		"MEMPOOL.",
	}
	lockLabelPrefixes = []string{
		"LCK.",
	}
	memLabelPrefixes = []string{
		"SMA.",
	}
	managementLabelPrefixes = []string{
		"MGT.",
	}
)

// Always returns at least one main label
func prometheusGroup(metric *varnishMetric) string {
	if startsWithAny(metric.Name, backendLabelPrefixes, caseSensitive) {
		return "backend"
	} else if startsWithAny(metric.Name, mempoolLabelPrefixes, caseSensitive) {
		return "mempool"
	} else if startsWithAny(metric.Name, lockLabelPrefixes, caseSensitive) {
		return "lock"
	} else if startsWithAny(metric.Name, memLabelPrefixes, caseSensitive) {
		return "memory"
	} else if startsWithAny(metric.Name, managementLabelPrefixes, caseSensitive) {
		return "management"
	}
	return "main"
}

var (
	fetchLabelPostfixes = []string{
		"fetch_1xx",
		"fetch_204",
		"fetch_304",
	}
)

func prometheusExtraLabels(metric *varnishMetric) prometheus.Labels {
	labels := make(prometheus.Labels)
	if len(metric.Identifier) > 0 {
		labels["ident"] = metric.Identifier
	}
	if endsWithAny(metric.Name, fetchLabelPostfixes, caseSensitive) {
		labels["code"] = metric.Name[len(metric.Name)-3:]
	}
	if len(labels) == 0 {
		return nil
	}
	return labels
}
