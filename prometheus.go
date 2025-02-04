package main

import (
	"regexp"
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
	if pe.version != nil {
		ch <- pe.version.Desc()
	}

	if StartParams.Verbose {
		logInfo("prometheus.Collector.Describe  %s", time.Now().Sub(start))
	}
}

// Implements prometheus.Collector
func (pe *prometheusExporter) Collect(ch chan<- prometheus.Metric) {
	start := time.Now()

	pe.Lock()
	defer pe.Unlock()

	// Rare case of varnish not being installed in the system
	// when we started, but installed while we are running.
	if !VarnishVersion.Valid() {
		if VarnishVersion.Initialize() == nil {
			pe.version = prometheus.NewGauge(prometheus.GaugeOpts{
				Namespace:   exporterNamespace,
				Name:        "version",
				Help:        "Varnish version information",
				ConstLabels: VarnishVersion.Labels(),
			})
		}
	}

	hadError := ExitHandler.HasError()

	_, err := ScrapeVarnish(ch)
	ExitHandler.Set(err)

	if err == nil {
		if hadError {
			logInfo("Successful scrape")
		}
		pe.up.Set(1)
	} else {
		pe.up.Set(0)
	}

	ch <- pe.up
	if pe.version != nil {
		ch <- pe.version
	}

	if StartParams.Verbose {
		postfix := ""
		if err != nil {
			postfix = " (scrape failed)"
		}
		logInfo("prometheus.Collector.Collect   %s%s", time.Now().Sub(start), postfix)
	}
}

// utils

type group struct {
	name     string
	prefixes []string
}

var (
	groups = []group{
		{name: "backend", prefixes: []string{
			"vbe.",
		}},
		{name: "mempool", prefixes: []string{
			"mempool.",
		}},
		{name: "lck", prefixes: []string{
			"lck.",
		}},
		{name: "sma", prefixes: []string{
			"sma.",
		}},
		{name: "smf", prefixes: []string{
			"smf.",
		}},
		{name: "mgt", prefixes: []string{
			"mgt.",
		}},
		{name: "main", prefixes: []string{
			"main.",
		}},
	}
)

type grouping struct {
	newPrefix string
	prefix    string
	total     string
	desc      string
	labelKey  string
}

var (
	fqGroupPrefixes = []*grouping{
		{
			prefix: "main_fetch",
			total:  "main_s_fetch",
			desc:   "Number of fetches",
		},
		{
			newPrefix: "main_sessions",
			prefix:    "main_sess",
			total:     "main_s_sess",
			desc:      "Number of sessions",
		},
		{
			newPrefix: "main_worker_threads",
			prefix:    "main_n_wrk",
			total:     "main_n_wrk",
			desc:      "Number of worker threads",
		},
	}
	fqNames = map[string]string{
		"varnish_lck_colls":   "varnish_lock_collisions",
		"varnish_lck_creat":   "varnish_lock_created",
		"varnish_lck_destroy": "varnish_lock_destroyed",
		"varnish_lck_locks":   "varnish_lock_operations",
	}
	fqIdentifiers = map[string]string{
		"varnish_lock_collisions": "target",
		"varnish_lock_created":    "target",
		"varnish_lock_destroyed":  "target",
		"varnish_lock_operations": "target",
		"varnish_sma_c_bytes":     "type",
		"varnish_sma_c_fail":      "type",
		"varnish_sma_c_freed":     "type",
		"varnish_sma_c_req":       "type",
		"varnish_sma_g_alloc":     "type",
		"varnish_sma_g_bytes":     "type",
		"varnish_sma_g_space":     "type",
		"varnish_smf_c_bytes":     "type",
		"varnish_smf_c_fail":      "type",
		"varnish_smf_c_freed":     "type",
		"varnish_smf_c_req":       "type",
		"varnish_smf_g_alloc":     "type",
		"varnish_smf_g_bytes":     "type",
		"varnish_smf_g_smf_frag":  "type",
		"varnish_smf_g_smf_large": "type",
		"varnish_smf_g_smf":       "type",
		"varnish_smf_g_space":     "type",
	}
)

var (
	// (prefix:)<uuid>.<name>
	regexBackendUUID = regexp.MustCompile(`([[0-9A-Za-z]{8}-[0-9A-Za-z]{4}-[0-9A-Za-z]{4}-[89ABab][0-9A-Za-z]{3}-[0-9A-Za-z]{12})(.*)`)
	// <name>(<ip>,(<something>),<port>)
	regexBackendParen = regexp.MustCompile(`(.*)\((.*)\)`)
)

func findLabelValue(name string, keys, values []string) string {
	for i, key := range keys {
		if key == name {
			if i < len(values) {
				return values[i]
			}
			return ""
		}
	}
	return ""
}

func cleanBackendName(name string) string {
	name = strings.Trim(name, ".")
	for _, prefix := range []string{"boot.", "root:"} {
		if startsWith(name, prefix, caseInsensitive) {
			name = name[len(prefix):]
		}
	}

	// reload_2019-08-29T100458.<name> as by varnish_reload_vcl in 4.x
	// reload_20191014_091124_78599.<name> as by varnishreload in 6+
	if strings.HasPrefix(name, "reload_") {
		dot := strings.Index(name, ".")
		if dot != -1 {
			name = name[dot+1:]
		}
	}

	return name
}

// https://prometheus.io/docs/practices/naming/
func computePrometheusInfo(vName, vGroup, vIdentifier, vDescription string) (name, description string, labelKeys, labelValues []string) {
	{
		// Varnish >= 5.2 no longer has 'ident', parse from full vName
		// as "<group>.<ident>.<name>"
		if len(vIdentifier) == 0 && strings.Count(vName, ".") > 1 {
			vIdentifier = prometheusTrimGroupPrefix(strings.ToLower(vName))
			vIdentifier = vIdentifier[0:strings.LastIndex(vIdentifier, ".")]
		}
	}
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
		if swapName := fqNames[name]; len(swapName) > 0 {
			name = swapName
		}
		description = vDescription
	}
	// labels: can alter final name and description
	{
		if len(vIdentifier) > 0 {
			if isVBE := startsWith(vName, "VBE.", caseSensitive); isVBE {
				if hits := regexBackendUUID.FindAllStringSubmatch(vIdentifier, -1); len(hits) > 0 && len(hits[0]) >= 3 {
					labelKeys, labelValues = append(labelKeys, "backend"), append(labelValues, cleanBackendName(hits[0][2]))
					labelKeys, labelValues = append(labelKeys, "server"), append(labelValues, hits[0][1])
				} else if hits := regexBackendParen.FindAllStringSubmatch(vIdentifier, -1); len(hits) > 0 && len(hits[0]) >= 3 {
					labelKeys, labelValues = append(labelKeys, "backend"), append(labelValues, cleanBackendName(hits[0][1]))
					labelKeys, labelValues = append(labelKeys, "server"), append(labelValues, strings.Replace(hits[0][2], ",,", ":", 1))
				}
				// We must be consistent with the number of labels and their names inside this scrape and between scrapes, or we will get this error:
				// https://github.com/prometheus/client_golang/blob/3fb8ace93bc4ccddea55af62320c2fd109252880/prometheus/registry.go#L704-L707
				if len(labelKeys) == 0 {
					labelKeys, labelValues = append(labelKeys, "backend"), append(labelValues, cleanBackendName(vIdentifier))
					labelKeys, labelValues = append(labelKeys, "server"), append(labelValues, "unknown")
				}
			}
			if len(labelKeys) == 0 {
				labelKey := fqIdentifiers[name]
				if len(labelKey) == 0 {
					labelKey = "id"
				}
				labelKeys, labelValues = append(labelKeys, labelKey), append(labelValues, vIdentifier)
			}
		}

		// create groupings by moving part of the fq name as a label and optional total
		for _, grouping := range fqGroupPrefixes {
			fqTotal := exporterNamespace + "_" + grouping.total
			fqPrefix := exporterNamespace + "_" + grouping.prefix
			fqNewName := fqPrefix
			if len(grouping.newPrefix) > 0 {
				fqNewName = exporterNamespace + "_" + grouping.newPrefix
			}
			if name == fqTotal {
				// @note total should not be a label as it breaks aggregation
				name, description = fqNewName+"_total", grouping.desc
				break
			} else if len(name) > len(fqPrefix)+1 && strings.HasPrefix(name, fqPrefix+"_") {
				labelKey := "type"
				if len(grouping.labelKey) > 0 {
					labelKey = grouping.labelKey
				}
				labelKeys, labelValues = append(labelKeys, labelKey), append(labelValues, name[len(fqPrefix)+1:])
				name, description = fqNewName, grouping.desc
				break
			}
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
