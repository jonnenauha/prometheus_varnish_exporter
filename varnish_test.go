package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func Test_VarnishVersion(t *testing.T) {
	tests := map[string]*varnishVersion{
		"varnishstat (varnish-6.0.0 revision a068361dff0d25a0d85cf82a6e5fdaf315e06a7d)": &varnishVersion{
			Major: 6, Minor: 0, Patch: 0, Revision: "a068361dff0d25a0d85cf82a6e5fdaf315e06a7d",
		},
		"varnishstat (varnish-5.2.0 revision 4c4875cbf)": &varnishVersion{
			Major: 5, Minor: 2, Patch: 0, Revision: "4c4875cbf",
		},
		"varnishstat (varnish-4.1.10 revision 1d090c5a08f41c36562644bafcce9d3cb85d824f)": &varnishVersion{
			Major: 4, Minor: 1, Patch: 10, Revision: "1d090c5a08f41c36562644bafcce9d3cb85d824f",
		},
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
		if !test.EqualsOrGreater(test.Major, test.Minor) {
			t.Fatalf("%s does not satisfy itself", test)
		}
		if !test.EqualsOrGreater(test.Major-1, 0) {
			t.Fatalf("%s should satisfy version %d.0", test, test.Major-1)
		}
		if test.EqualsOrGreater(test.Major, test.Minor+1) {
			t.Fatalf("%s should not satisfy version %d.%d", test, test.Major, test.Minor+1)
		}
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

func matchStringSlices(s1, s2 []string) bool {
	if len(s1) != len(s2) {
		return false
	}
	for i, v1 := range s1 {
		if s2[i] != v1 {
			return false
		}
	}
	return true
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
		// Varnish < 5.2
		name_1, _, labelKeys_1, labelValues_1 := computePrometheusInfo(vName, vGroup, vIdentifier, vDescription)
		t.Logf("%s > %s > %s\n", vName, backend, name_1)
		t.Logf("  ident   : %s\n", vIdentifier)
		t.Logf("  backend : %s\n", findLabelValue("backend", labelKeys_1, labelValues_1))
		t.Logf("  server  : %s\n", findLabelValue("server", labelKeys_1, labelValues_1))

		// Varnish >= 5.2 no longer has 'ident', test that detected correctly from vName
		name_2, _, labelKeys_2, labelValues_2 := computePrometheusInfo(vName, vGroup, "", vDescription)
		if name_1 != name_2 {
			t.Fatalf("name %q != %q", name_1, name_2)
		}
		if !matchStringSlices(labelKeys_1, labelKeys_2) {
			t.Fatalf("labelKeys %#v != %#v", labelKeys_1, labelKeys_2)
		}
		if !matchStringSlices(labelValues_1, labelValues_2) {
			t.Fatalf("labelKeys %#v != %#v", labelValues_1, labelValues_2)
		}
	}
}

func Test_VarnishMetrics(t *testing.T) {
	dir, _ := os.Getwd()
	if !fileExists(filepath.Join(dir, "test/scrape")) {
		t.Skipf("Cannot find test/scrape files from workind dir %s", dir)
	}
	for _, test := range []string{
		filepath.Join(dir, "test/scrape", "4.1.1.json"),
		filepath.Join(dir, "test/scrape", "5.2.0.json"),
		filepath.Join(dir, "test/scrape", "6.0.0.json"),
	} {
		version := strings.Replace(filepath.Base(test), ".json", "", -1)
		VarnishVersion.parseVersion(version)
		t.Logf("test scrape %s", VarnishVersion)

		buf, err := ioutil.ReadFile(test)
		if err != nil {
			t.Fatal(err.Error())
		}
		metrics := make(chan prometheus.Metric)
		descs := []*prometheus.Desc{}
		go func() {
			for m := range metrics {
				descs = append(descs, m.Desc())
			}
		}()
		_, err = ScrapeVarnishFrom(buf, metrics)
		close(metrics)

		if err != nil {
			t.Fatal(err.Error())
		}
		t.Logf("  %d metrics", len(descs))
	}
}

// Testing against a live varnish instance is only executed in build bot(s).
// This is because the usual end user setup requires tests to be ran with sudo in order to work.
func Test_VarnishMetrics_CI(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skipf("Host needs to be linux to run live metrics test: %s", runtime.GOOS)
		return
	} else if os.Getenv("CONTINUOUS_INTEGRATION") != "true" {
		t.Skip("Live metrics test only ran on CI")
		return
	}

	StartParams.Verbose = true
	StartParams.Raw = true

	if err := VarnishVersion.Initialize(); err != nil {
		t.Fatal(err)
	}

	metrics := make(chan prometheus.Metric)
	go func() {
		for m := range metrics {
			t.Logf("%s", m.Desc())
		}
	}()
	if _, err := ScrapeVarnish(metrics); err != nil {
		t.Fatal(err)
	}
	close(metrics)
}
