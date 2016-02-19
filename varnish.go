package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

const (
	varnishstatExe = "varnishstat"
)

var (
	VarnishExporter = &varnishExporter{}

	ignoredLinesList = []string{
		"varnishstat",
		"field name",
		"-----",
	}
)

type varnishMetric struct {
	Name               string
	Value              float64
	Description        string
	Identifier         string
	Type, Flag, Format string
}

type varnishVersion struct {
	major    int
	minor    int
	patch    int
	revision string
}

func NewVarnishVersion() *varnishVersion {
	return &varnishVersion{
		major: -1,
		minor: -1,
		patch: -1,
	}
}

func (v *varnishVersion) set(parts []string) error {
	for i, part := range parts {
		if len(part) == 0 {
			continue
		}
		if i == 3 {
			v.revision = part
			break
		}
		num, err := strconv.Atoi(part)
		if err != nil {
			return err
		}
		switch i {
		case 0:
			v.major = num
		case 1:
			v.minor = num
		case 2:
			v.patch = num
		}
	}
	return nil
}

func (v *varnishVersion) isValid() bool {
	return v.major != -1
}

func (v *varnishVersion) String() string {
	parts := []string{}
	for _, num := range []int{v.major, v.minor, v.patch} {
		if num != -1 {
			parts = append(parts, strconv.Itoa(num))
		}
	}
	version := strings.Join(parts, ".")
	if v.revision != "" {
		version += " " + v.revision
	}
	return version
}

type varnishExporter struct {
	version *varnishVersion
	metrics []*varnishMetric
}

func (v *varnishExporter) queryVersion() error {
	buf := &bytes.Buffer{}
	cmd := exec.Command(varnishstatExe, "-V")
	cmd.Stdout = buf
	cmd.Stderr = buf
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := cmd.Wait(); err != nil {
		return err
	}
	scanner := bufio.NewScanner(buf)
	for scanner.Scan() {
		return v.parseVersion(scanner.Text())

	}
	return nil
}

func (v *varnishExporter) parseVersion(version string) error {
	v.version = NewVarnishVersion()

	r := regexp.MustCompile(`(\d)\.?(\d)?\.?(\d)?(?:.*revision\s(.*)\))?`)
	parts := r.FindStringSubmatch(version)
	if len(parts) > 1 {
		if err := v.version.set(parts[1:]); err != nil {
			return err
		}
	}
	if !v.version.isValid() {
		return fmt.Errorf("Failed to resolve version from %q", version)
	}
	return nil
}

func (v *varnishExporter) queryMetrics() error {
	buf := &bytes.Buffer{}
	cmd := exec.Command(varnishstatExe, "-j")
	cmd.Stdout = buf
	cmd.Stderr = buf
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := cmd.Wait(); err != nil {
		return err
	}
	return v.parseMetrics(buf)
}

func (v *varnishExporter) parseMetrics(r io.Reader) error {
	// The output JSON annoyingly is not stuctured so that we could make a nice struct for it.
	// it has a 'timestamp' key
	var metrics map[string]interface{}
	dec := json.NewDecoder(r)
	if err := dec.Decode(&metrics); err != nil {
		return err
	}

	const timestamp = "timestamp"
	for name, raw := range metrics {
		if name == timestamp {
			continue
		}
		if dt := reflect.TypeOf(raw); dt.Kind() != reflect.Map {
			return fmt.Errorf("Found unexpected data from json: %q = %#v", name, raw)
		}
		data, ok := raw.(map[string]interface{})
		if !ok {
			return fmt.Errorf("Failed to cast to map[string]interface{}: %q = %#v", name, raw)
		}
		m := &varnishMetric{
			Name: name,
		}
		//fmt.Printf("%s = %#v\n", name, data)
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
				m.Value = value.(float64)
			default:
				// Test mode failure only, don't break future versions that might add more stuff
				if StartParams.Test {
					return fmt.Errorf("Unhandled property: %s = %#v", prop, value)
				}
			}
			//fmt.Printf("    %s = %#v\n", prop, value)
		}
		v.metrics = append(v.metrics, m)
	}
	if len(v.metrics) == 0 {
		return fmt.Errorf("No metrics found from %s output", varnishstatExe)
	}
	return nil
}

// @note deprecated in favor of JSON queries, might make a helpful library however if exported one day
func (v *varnishExporter) queryMetricsList() error {
	buf := &bytes.Buffer{}
	cmd := exec.Command(varnishstatExe, "-l")
	cmd.Stdout = buf
	cmd.Stderr = buf
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := cmd.Wait(); err != nil {
		return err
	}
	return v.parseMetricsList(buf)
}

// @note deprecated in favor of JSON queries, might make a helpful library however if exported one day
func (v *varnishExporter) parseMetricsList(r io.Reader) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 || startsWithAny(line, ignoredLinesList, caseInsensitive) {
			continue
		}
		if parts := strings.SplitAfterN(line, " ", 2); len(parts) == 2 {
			m := &varnishMetric{
				Name:        strings.TrimSpace(parts[0]),
				Description: strings.TrimSpace(parts[1]),
			}
			v.metrics = append(v.metrics, m)
		} else {
			logWarn("Found invalid metrics line: %q", line)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if len(v.metrics) == 0 {
		return fmt.Errorf("No metrics found from %s output", varnishstatExe)
	}
	return nil
}
