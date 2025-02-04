package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

var testFileVersions = []string{"3.0.5", "4.0.5", "4.1.1", "5.2.0", "6.0.0", "6.5.1"}

func Test_VarnishVersion(t *testing.T) {
	tests := map[string]*varnishVersion{
		"varnishstat (varnish-6.5.1 revision 1dae23376bb5ea7a6b8e9e4b9ed95cdc9469fb64)": {
			Major: 6, Minor: 5, Patch: 1, Revision: "1dae23376bb5ea7a6b8e9e4b9ed95cdc9469fb64",
		},
		"varnishstat (varnish-6.0.0 revision a068361dff0d25a0d85cf82a6e5fdaf315e06a7d)": {
			Major: 6, Minor: 0, Patch: 0, Revision: "a068361dff0d25a0d85cf82a6e5fdaf315e06a7d",
		},
		"varnishstat (varnish-5.2.0 revision 4c4875cbf)": {
			Major: 5, Minor: 2, Patch: 0, Revision: "4c4875cbf",
		},
		"varnishstat (varnish-4.1.10 revision 1d090c5a08f41c36562644bafcce9d3cb85d824f)": {
			Major: 4, Minor: 1, Patch: 10, Revision: "1d090c5a08f41c36562644bafcce9d3cb85d824f",
		},
		"varnishstat (varnish-4.1.0 revision 3041728)": {
			Major: 4, Minor: 1, Patch: 0, Revision: "3041728",
		},
		"varnishstat (varnish-4 revision)": {
			Major: 4, Minor: -1, Patch: -1,
		},
		"varnishstat (varnish-3.0.5 revision 1a89b1f)": {
			Major: 3, Minor: 0, Patch: 5, Revision: "1a89b1f",
		},
		"varnish 2.0": {
			Major: 2, Minor: 0, Patch: -1,
		},
		"varnish 1": {
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
	for _, variant := range [][]string{
		{"eu1_x.y-z:w(192.52.0.192,,8085)", "eu1_x.y-z:w", "192.52.0.192,,8085"}, // 4.0.3
		{"root:eu2_x.y-z:w", "eu2_x.y-z:w", "unknown"},                           // 4.1
		{"def0e7f7-a676-4eed-9d8b-78ef7ce21e93.us1_x.y-z:w", "us1_x.y-z:w", "def0e7f7-a676-4eed-9d8b-78ef7ce21e93"},
		{"root:29813cbb-7329-4eb8-8969-26be2ef58c88.us2_x.y-z:w", "us2_x.y-z:w", "29813cbb-7329-4eb8-8969-26be2ef58c88"}, // ??
		{"boot.default", "default", "unknown"},
		{"reload_2019-08-29T100458.default", "default", "unknown"},     // varnish_reload_vcl in 4
		{"reload_20191016_072034_54500.default", "default", "unknown"}, // varnishreload in 6+
		{"ce19737f-72b5-4f4b-9d39-3d8c2d28240b.default", "default", "ce19737f-72b5-4f4b-9d39-3d8c2d28240b"},
	} {
		backend := variant[0]
		expected_backend := variant[1]
		expected_server := variant[2]

		vName, data := dummyBackendValue(backend)
		var (
			vGroup       = prometheusGroup(vName)
			vDescription string
			vIdentifier  string
			vErr         error
		)
		if value, ok := data["description"]; ok {
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
		computed_backend := findLabelValue("backend", labelKeys_1, labelValues_1)
		computed_server := findLabelValue("server", labelKeys_1, labelValues_1)
		t.Logf("%s > %s > %s\n", vName, backend, name_1)
		t.Logf("  ident   : %s\n", vIdentifier)
		t.Logf("  backend : %s\n", computed_backend)
		t.Logf("  server  : %s\n", computed_server)

		if expected_backend != computed_backend {
			t.Fatalf("backend %q != %q", computed_backend, expected_backend)
		}
		if expected_server != expected_server {
			t.Fatalf("server %q != %q", computed_server, expected_server)
		}

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
	for _, version := range testFileVersions {
		test := filepath.Join(dir, "test/scrape", version+".json")
		VarnishVersion.parseVersion(version)
		t.Logf("test scrape %s", VarnishVersion)

		buf, err := ioutil.ReadFile(test)
		if err != nil {
			t.Fatal(err.Error())
		}
		done := make(chan bool)
		metrics := make(chan prometheus.Metric)
		descs := []*prometheus.Desc{}
		go func() {
			for m := range metrics {
				descs = append(descs, m.Desc())
			}
			done <- true
		}()
		_, err = ScrapeVarnishFrom(buf, metrics)
		close(metrics)
		<-done

		if err != nil {
			t.Fatal(err.Error())
		}
		t.Logf("  %d metrics", len(descs))
	}
}

func Test_FindMostRecentVbeReloadPrefix(t *testing.T) {
	type testConfig struct {
		varnishCounters                   map[string]interface{}
		expectedMostRecentVbeReloadPrefix string
	}

	for _, testConfig := range []testConfig{
		// Varnish <= 4.0 has no duplicated stats on reload
		{map[string]interface{}{
			"VBE.default(127.0.0.1,,8080).happy": "any",
		}, ""},
		// Varnish 4.1 or later, not yet reloaded
		{map[string]interface{}{
			"VBE.boot.default.happy": "any",
		}, ""},
		// Varnish 4.1, reloaded 2 times
		{map[string]interface{}{
			"VBE.boot.default.happy":                     "any",
			"VBE.reload_2019-08-29T100458.default.happy": "any",
			"VBE.reload_2019-08-29T100459.default.happy": "any",
		}, "VBE.reload_2019-08-29T100459"},
		// Varnish 6+, reloaded 2 times
		{map[string]interface{}{
			"VBE.boot.default.happy":                         "any",
			"VBE.reload_20191016_072034_54500.default.happy": "any",
			"VBE.reload_20191016_072034_54501.default.happy": "any",
		}, "VBE.reload_20191016_072034_54501"},
	} {
		computedMostRecentVbeReloadPrefix := findMostRecentVbeReloadPrefix(testConfig.varnishCounters)
		t.Logf("Varnish counters: %s\n", testConfig.varnishCounters)
		t.Logf("  expected most recent prefix : '%s'\n", testConfig.expectedMostRecentVbeReloadPrefix)
		t.Logf("  computed most recent prefix : '%s'\n", computedMostRecentVbeReloadPrefix)

		if computedMostRecentVbeReloadPrefix != testConfig.expectedMostRecentVbeReloadPrefix {
			t.Fatalf("mostRecentVbeReloadPrefix %q != %q", computedMostRecentVbeReloadPrefix, testConfig.expectedMostRecentVbeReloadPrefix)
		}
	}
}

func Test_IsOutdatedVbe(t *testing.T) {
	type testConfig struct {
		vName                     string
		mostRecentVbeReloadPrefix string
		expectedIsOutdatedVbe     bool
	}

	for _, testConfig := range []testConfig{
		{"MGT.uptime", "", false},                                                                    // not VBE
		{"MGT.uptime", "VBE.reload_20191016_072034_54500", false},                                    // not VBE
		{"VBE.boot.default.conn", "", false},                                                         // VCL not yet reloaded
		{"VBE.boot.default.conn", "VBE.reload_20191016_072034_54500", true},                          // VCL reloaded, 'boot' is now outdated
		{"VBE.reload_20191016_072034_54500.default.conn", "VBE.reload_20191016_072034_54500", false}, // current VCL version
		{"VBE.reload_20191016_072034_54499.default.conn", "VBE.reload_20191016_072034_54500", true},  // previous VCL version
	} {
		computedIsOutdatedVbe := isOutdatedVbe(testConfig.vName, testConfig.mostRecentVbeReloadPrefix)
		t.Logf("'%s', '%s'\n", testConfig.vName, testConfig.mostRecentVbeReloadPrefix)
		t.Logf("  expected outdated : %t\n", testConfig.expectedIsOutdatedVbe)
		t.Logf("  computed outdated : %t\n", computedIsOutdatedVbe)

		if computedIsOutdatedVbe != testConfig.expectedIsOutdatedVbe {
			t.Fatalf("outdatedVbe %t != %t", computedIsOutdatedVbe, testConfig.expectedIsOutdatedVbe)
		}
	}
}

type testCollector struct {
	filepath string
	t        *testing.T
}

func (tc *testCollector) Describe(ch chan<- *prometheus.Desc) {
}

func (tc *testCollector) Collect(ch chan<- prometheus.Metric) {
	buf, err := os.ReadFile(tc.filepath)
	if err != nil {
		tc.t.Fatal(err.Error())
	}
	_, err = ScrapeVarnishFrom(buf, ch)

	if err != nil {
		tc.t.Fatal(err.Error())
	}
}

func Test_PrometheusExport(t *testing.T) {
	dir, _ := os.Getwd()
	if !fileExists(filepath.Join(dir, "test/scrape")) {
		t.Skipf("Cannot find test/scrape files from workind dir %s", dir)
	}
	for _, version := range testFileVersions {
		test := filepath.Join(dir, "test/scrape", version+".json")
		VarnishVersion.parseVersion(version)
		t.Logf("test scrape %s", VarnishVersion)

		registry := prometheus.NewRegistry()
		collector := &testCollector{filepath: test, t: t}
		registry.MustRegister(collector)

		gathering, err := registry.Gather()
		if err != nil {
			errors, ok := err.(prometheus.MultiError)
			if ok {
				for _, e := range errors {
					t.Errorf("  Error in prometheus Gather: %#v", e)
				}
			} else {
				t.Errorf("  Error in prometheus Gather: %#v", err)
			}
		}

		metricCount := 0

		includesVarnishBackendHappy := false
		for _, mf := range gathering {
			if *mf.Name == "varnish_backend_happy" {
				includesVarnishBackendHappy = true
			}

			metricCount += len(mf.Metric)
		}

		// Validate presence of a single metric, which is present across all versions
		if !includesVarnishBackendHappy {
			t.Error("Missing metric varnish_backend_happy")
		}

		t.Logf("  %d metrics", metricCount)
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

	done := make(chan bool)
	metrics := make(chan prometheus.Metric)
	go func() {
		for m := range metrics {
			t.Logf("%s", m.Desc())
		}
		done <- true
	}()
	if _, err := ScrapeVarnish(metrics); err != nil {
		t.Fatal(err)
	}
	close(metrics)
	<-done
}
