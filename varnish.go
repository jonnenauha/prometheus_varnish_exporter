package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	varnishstatExe = "varnishstat"
)

var (
	descCache  = make(map[string]*prometheus.Desc)
	mDescCache sync.RWMutex
)

func scrapeVarnish(ch chan<- prometheus.Metric) error {
	params := []string{"-j"}
	if VarnishVersion.Major >= 4 {
		// timeout to not hang for a long time if instance is not found.
		// Varnish 3.x exits immediately on faulty params
		params = append(params, "-t", "2")
	}
	if !StartParams.Params.isEmpty() {
		params = append(params, StartParams.Params.make()...)
	}
	buf, errExec := executeVarnishstat(params...)
	if errExec != nil {
		return errExec
	}
	// The output JSON annoyingly is not stuctured so that we could make a nice map[string]struct for it.
	metricsJSON := make(map[string]interface{})
	dec := json.NewDecoder(buf)
	if err := dec.Decode(&metricsJSON); err != nil {
		return err
	}

	// This is a bit broad but better than locking on each desc query below.
	mDescCache.Lock()
	defer mDescCache.Unlock()

	for vName, raw := range metricsJSON {
		if vName == "timestamp" {
			continue
		}
		if dt := reflect.TypeOf(raw); dt.Kind() != reflect.Map {
			if StartParams.Verbose {
				logWarn("Found unexpected data from json: %s: %#v", vName, raw)
			}
			continue
		}
		data, ok := raw.(map[string]interface{})
		if !ok {
			if StartParams.Verbose {
				logWarn("Failed to cast to map[string]interface{}: %s: %#v", vName, raw)
			}
			continue
		}
		var (
			vGroup       = prometheusGroup(vName)
			vDescription string
			vIdentifier  string
			vValue       float64
			vErr         error
		)
		if value, ok := data["description"]; ok && vErr == nil {
			if vDescription, ok = value.(string); !ok {
				vErr = fmt.Errorf("%s description it not a string", vName)
			}
		}
		if value, ok := data["ident"]; ok && vErr == nil {
			if vIdentifier, ok = value.(string); !ok {
				vErr = fmt.Errorf("%s ident it not a string", vName)
			}
		}
		if value, ok := data["value"]; ok && vErr == nil {
			if vValue, ok = value.(float64); !ok {
				vErr = fmt.Errorf("%s value it not a float64", vName)
			}
		}
		if vErr != nil {
			if StartParams.Verbose {
				logWarn(vErr.Error())
			}
			continue
		}

		pName, pDescription, pLabelKeys, pLabelValues := computePrometheusInfo(vName, vGroup, vIdentifier, vDescription)

		descKey := pName + "_" + strings.Join(pLabelKeys, "_")
		pDesc, ok := descCache[descKey]
		if !ok {
			pDesc = prometheus.NewDesc(
				pName,
				pDescription,
				pLabelKeys,
				nil,
			)
			descCache[descKey] = pDesc
		}
		valueType := prometheus.GaugeValue
		if swapValueType, ok := fqMetricType[pName]; ok {
			valueType = swapValueType
		}
		ch <- prometheus.MustNewConstMetric(pDesc, valueType, vValue, pLabelValues...)
	}
	return nil
}

// Returns the result of 'varnishtat' with optional command line params.
func executeVarnishstat(params ...string) (*bytes.Buffer, error) {
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
	buf, err := executeVarnishstat("-V")
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
