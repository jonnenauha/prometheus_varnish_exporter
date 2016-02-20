package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

const (
	varnishstatExe = "varnishstat"
)

// Utils

// Returns the result of 'varnishtat' with optional command line params.
func ExecuteVarnishstat(params ...string) (*bytes.Buffer, error) {
	buf := bytes.Buffer{}
	cmd := exec.Command(varnishstatExe, params...)
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	if err := cmd.Wait(); err != nil {
		return nil, err
	}
	return &buf, nil
}

// varnishMetric

type varnishMetric struct {
	Name               string
	Value              float64
	Description        string
	Identifier         string
	Type, Flag, Format string
}

// varnishExporter

type varnishExporter struct {
	sync.RWMutex

	metrics       []*varnishMetric
	metricsByName map[string]*varnishMetric
}

func NewVarnishExporter() *varnishExporter {
	return &varnishExporter{
		metricsByName: make(map[string]*varnishMetric),
	}
}

func (v *varnishExporter) MetricByName(name string) *varnishMetric {
	v.RLock()
	m := v.metricsByName[name]
	v.RUnlock()
	return m
}

// Initializes exporter.
func (v *varnishExporter) Initialize() (err error) {
	v.Lock()
	defer v.Unlock()

	v.metrics, err = v.queryMetrics()
	if err == nil && len(v.metrics) == 0 {
		return fmt.Errorf("No metrics found from %s output", varnishstatExe)
	}
	v.metricsByName = make(map[string]*varnishMetric)
	for _, m := range v.metrics {
		v.metricsByName[m.Name] = m
	}
	if len(v.metricsByName) != len(v.metrics) {
		return fmt.Errorf("Metrics count mismatch after mapping: map:%d flat:%d", len(v.metricsByName), len(v.metrics))
	}
	return nil
}

// Updates all metrics from varnishstat.
func (v *varnishExporter) Update() error {
	v.Lock()
	defer v.Unlock()

	// query process
	if len(v.metrics) == 0 {
		return errors.New("varnishExporter.Collect: no metrics to update")
	}
	buf, err := ExecuteVarnishstat("-j")
	if err != nil {
		return err
	}

	// The output JSON annoyingly is not stuctured so that we could make a nice struct for it.
	// it has a 'timestamp' key
	// @todo slight code duplication with parseMetrics, this is a bit slimmed down impl though.
	var metricsJSON map[string]interface{}
	dec := json.NewDecoder(buf)
	if errDec := dec.Decode(&metricsJSON); errDec != nil {
		return errDec
	}

	const timestamp = "timestamp"
	for name, raw := range metricsJSON {
		if name == timestamp {
			continue
		}
		m := v.metricsByName[name]
		if m == nil {
			// @todo Shedule a new init query and recreate internal state if
			// more stats have emerged after service startup.
			logWarn("Failed to find existing metric for %q", name)
			continue
		}
		// @note We can skip the reflect type check and cast validation here.
		// They were executed in parseMetrics and if failed, would never end up
		// int metricsByName.
		data := raw.(map[string]interface{})

		// We are only interested in the new value for updating existing metrics.
		// Type is float64 or there would have been an error in parseMetrics.
		if value, ok := data["value"]; ok {
			m.Value = value.(float64)
		}
	}
	return nil
}

// Initial query at startup to resolve available metrics.
func (v *varnishExporter) queryMetrics() ([]*varnishMetric, error) {
	buf, err := ExecuteVarnishstat("-j")
	if err != nil {
		return nil, err
	}
	return v.parseMetrics(buf)
}

// Parse new metrics.
func (v *varnishExporter) parseMetrics(r io.Reader) ([]*varnishMetric, error) {
	// The output JSON annoyingly is not stuctured so that we could make a nice struct for it.
	// it has a 'timestamp' key
	var metricsJSON map[string]interface{}
	dec := json.NewDecoder(r)
	if err := dec.Decode(&metricsJSON); err != nil {
		return nil, err
	}

	const timestamp = "timestamp"
	var metrics []*varnishMetric
	for name, raw := range metricsJSON {
		if name == timestamp {
			continue
		}
		if dt := reflect.TypeOf(raw); dt.Kind() != reflect.Map {
			return nil, fmt.Errorf("Found unexpected data from json: %q = %#v", name, raw)
		}
		data, ok := raw.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("Failed to cast to map[string]interface{}: %q = %#v", name, raw)
		}
		m := &varnishMetric{
			Name: name,
		}
		for prop, value := range data {
			switch prop {
			case "description":
				m.Description = value.(string)
			case "ident":
				m.Identifier = value.(string)
			case "type":
				m.Type = value.(string)
			case "flag":
				m.Flag = value.(string)
			case "format":
				m.Format = value.(string)
			case "value":
				m.Value, ok = value.(float64)
				if !ok {
					return nil, fmt.Errorf("Non float64 property value: %s = %#v", prop, value)
				}
			default:
				// Test mode failure only, don't break future versions that might add more stuff
				if StartParams.Test {
					return nil, fmt.Errorf("Unhandled property: %s = %#v", prop, value)
				}
			}
		}
		metrics = append(metrics, m)
	}
	return metrics, nil
}

// varnishVersion

type varnishVersion struct {
	Major    int
	Minor    int
	Patch    int
	Revision string
}

func NewVarnishVersion() *varnishVersion {
	return &varnishVersion{
		Major: -1, Minor: -1, Patch: -1,
	}
}

func (v *varnishVersion) Initialize() error {
	return v.queryVersion()
}

func (v *varnishVersion) queryVersion() error {
	buf, err := ExecuteVarnishstat("-V")
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(buf)
	for scanner.Scan() {
		return v.parseVersion(scanner.Text())
	}
	return nil
}

func (v *varnishVersion) parseVersion(version string) error {
	r := regexp.MustCompile(`(\d)\.?(\d)?\.?(\d)?(?:.*revision\s(.*)\))?`)
	parts := r.FindStringSubmatch(version)
	if len(parts) > 1 {
		if err := v.set(parts[1:]); err != nil {
			return err
		}
	}
	if !v.isValid() {
		return fmt.Errorf("Failed to resolve version from %q", version)
	}
	return nil
}

func (v *varnishVersion) Labels() map[string]string {
	labels := make(map[string]string)
	if v.Major != -1 {
		labels["major"] = strconv.Itoa(v.Major)
	}
	if v.Minor != -1 {
		labels["minor"] = strconv.Itoa(v.Minor)
	}
	if v.Patch != -1 {
		labels["patch"] = strconv.Itoa(v.Patch)
	}
	if v.Revision != "" {
		labels["revision"] = v.Revision
	}
	labels["version"] = v.VersionString()
	return labels
}

func (v *varnishVersion) set(parts []string) error {
	for i, part := range parts {
		if len(part) == 0 {
			continue
		}
		if i == 3 {
			v.Revision = part
			break
		}
		num, err := strconv.Atoi(part)
		if err != nil {
			return err
		}
		switch i {
		case 0:
			v.Major = num
		case 1:
			v.Minor = num
		case 2:
			v.Patch = num
		}
	}
	return nil
}

func (v *varnishVersion) isValid() bool {
	return v.Major != -1
}

// Version string with numbers only, no revision.
func (v *varnishVersion) VersionString() string {
	parts := []string{}
	for _, num := range []int{v.Major, v.Minor, v.Patch} {
		if num != -1 {
			parts = append(parts, strconv.Itoa(num))
		}
	}
	return strings.Join(parts, ".")
}

// Full version string, including revision.
func (v *varnishVersion) String() string {
	version := v.VersionString()
	if v.Revision != "" {
		version += " " + v.Revision
	}
	return version
}
