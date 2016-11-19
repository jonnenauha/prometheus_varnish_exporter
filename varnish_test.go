package main

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func Test_VarnishVersion(t *testing.T) {
	tests := map[string]*varnishVersion{
		"varnishstat (varnish-4.1.0 revision 3041728)": &varnishVersion{
			Major: 4, Minor: 1, Patch: 0, Revision: "3041728",
		},
		"varnishstat (varnish-4 revision)": &varnishVersion{
			Major: 4, Minor: -1, Patch: -1,
		},
		"varnishstat (varnish-3.0.5 revision 1a89b1f)": &varnishVersion{
			Major: 3, Minor: 0, Patch: 5, Revision: "1a89b1f",
		},
		"varnish 2.0": &varnishVersion{
			Major: 2, Minor: 0, Patch: -1,
		},
		"varnish 1": &varnishVersion{
			Major: 1, Minor: -1, Patch: -1,
		},
	}
	for versionStr, test := range tests {
		v := NewVarnishVersion()
		if err := v.parseVersion(versionStr); err != nil {
			t.Error(err.Error())
			continue
		}
		if test.Major != v.Major ||
			test.Minor != v.Minor ||
			test.Patch != v.Patch ||
			test.Revision != v.Revision {
			t.Errorf("version mismatch on %q", versionStr)
			continue
		}
		t.Logf("%q > %s\n", versionStr, v.String())
	}
}

func dummyBackendValue(backend string) (string, map[string]interface{}) {
	return fmt.Sprintf("VBE.%s.happy", backend), map[string]interface{}{
		"description": "Happy health probes",
		"type":        "VBE",
		"ident":       backend,
		"flag":        "b",
		"format":      "b",
		"value":       0,
	}
}

func Test_VarnishBackendNames(t *testing.T) {
	for _, backend := range []string{
		"eu1_x.y-z:w(192.52.0.192,,8085)", // 4.0.3
		"root:eu2_x.y-z:w",                // 4.1
		"def0e7f7-a676-4eed-9d8b-78ef7ce21e93.us1_x.y-z:w",
		"root:29813cbb-7329-4eb8-8969-26be2ef58c88.us2_x.y-z:w", // ??
		"boot.default",
		"ce19737f-72b5-4f4b-9d39-3d8c2d28240b.default",
	} {
		vName, data := dummyBackendValue(backend)
		var (
			vGroup       = prometheusGroup(vName)
			vDescription string
			vIdentifier  string
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
		if vErr != nil {
			t.Error(vErr)
			return
		}
		name, _, labelKeys, labelValues := computePrometheusInfo(vName, vGroup, vIdentifier, vDescription)
		t.Logf("%s > %s\n", backend, name)
		t.Logf("  ident   : %s\n", vIdentifier)
		t.Logf("  backend : %s\n", findLabelValue("backend", labelKeys, labelValues))
		t.Logf("  server  : %s\n", findLabelValue("server", labelKeys, labelValues))
	}
}

func Test_VarnishMetrics(t *testing.T) {
	// @todo This is kind of pointless. The idea was to test against
	// JSON output of different versions of Varnish. Enable back at some point
	// and figure out a way to scrape from buffer without code duplication.
	if runtime.GOOS != "linux" {
		t.Skipf("Host needs to be linux to run metrics test: %s", runtime.GOOS)
		return
	}

	StartParams.Verbose = true
	StartParams.Raw = true

	metrics := make(chan prometheus.Metric)
	go func() {
		for m := range metrics {
			t.Logf("%s", m.Desc())
		}
	}()
	if _, err := scrapeVarnish(metrics); err != nil {
		t.Skipf("Host machine needs varnishstat to be able to run tests: %s", err.Error())
	}
	close(metrics)
}
