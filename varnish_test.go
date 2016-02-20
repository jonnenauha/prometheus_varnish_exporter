package main

import (
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
	if err := scrapeVarnish(metrics); err != nil {
		t.Skipf("Host machine needs varnishstat to be able to run tests: %s", err.Error())
	}
	close(metrics)
}
