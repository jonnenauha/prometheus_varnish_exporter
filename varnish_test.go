package main

import (
	"bytes"
	"testing"
)

func Test_VarnishVersion(t *testing.T) {
	tests := map[string]*varnishVersion{
		"varnishstat (varnish-4.1.0 revision 3041728)": &varnishVersion{
			major: 4, minor: 1, patch: 0, revision: "3041728",
		},
		"varnishstat (varnish-4 revision)": &varnishVersion{
			major: 4, minor: -1, patch: -1,
		},
		"varnishstat (varnish-3.0.5 revision 1a89b1f)": &varnishVersion{
			major: 3, minor: 0, patch: 5, revision: "1a89b1f",
		},
		"varnish 1.0": &varnishVersion{
			major: 1, minor: 0, patch: -1,
		},
	}
	for versionStr, test := range tests {
		var exporter varnishExporter
		if err := exporter.parseVersion(versionStr); err != nil {
			t.Error(err.Error())
			continue
		}
		if test.major != exporter.version.major ||
			test.minor != exporter.version.minor ||
			test.patch != exporter.version.patch ||
			test.revision != exporter.version.revision {
			t.Errorf("version mismatch on %q", versionStr)
			continue
		}
		t.Logf("%q > %s\n", versionStr, exporter.version.String())
	}
}

func Test_VarnishMetrics(t *testing.T) {
	jsons := [][]byte{
		// varnish 4.x
		[]byte(`
{
  "timestamp": "2016-02-19T20:40:51",
  "MAIN.uptime": {
    "description": "Child process uptime",
    "type": "MAIN", "flag": "c", "format": "d",
    "value": 8679967
  },
  "MAIN.sess_conn": {
    "description": "Sessions accepted",
    "type": "MAIN", "flag": "c", "format": "i",
    "value": 7401068
  },
  "MAIN.sess_drop": {
    "description": "Sessions dropped",
    "type": "MAIN", "flag": "c", "format": "i",
    "value": 0
  },
  "MAIN.sess_fail": {
    "description": "Session accept failures",
    "type": "MAIN", "flag": "c", "format": "i",
    "value": 0
  },
  "MEMPOOL.busyobj.live": {
    "description": "In use",
    "type": "MEMPOOL", "ident": "busyobj", "flag": "g", "format": "i",
    "value": 0
  },
  "VBE.91e82226-e891-458e-b7b8-13bdc0ccb1ee.mybackend1.happy": {
    "description": "Happy health probes",
    "type": "VBE", "ident": "91e82226-e891-458e-b7b8-13bdc0ccb1ee.mybackend1", "flag": "b", "format": "b",
    "value": 18446744073709551615
  },
  "VBE.11d82325-e891-458e-b7b8-13bdc0ccb1ee.mybackend2.bereq_hdrbytes": {
    "description": "Request header bytes",
    "type": "VBE", "ident": "11d82325-e891-458e-b7b8-13bdc0ccb1ee.mybackend2", "flag": "c", "format": "B",
    "value": 212723711
  }
}`),
		// varnish 3.x
		[]byte(`
{
	"timestamp": "2016-02-19T11:47:30",
	"client_conn": {"value": 0, "flag": "a", "description": "Client connections accepted"},
	"client_drop": {"value": 0, "flag": "a", "description": "Connection dropped, no sess/wrk"},
	"client_req": {"value": 0, "flag": "a", "description": "Client requests received"},
	"cache_hit": {"value": 0, "flag": "a", "description": "Cache hits"},
	"cache_hitpass": {"value": 0, "flag": "a", "description": "Cache hits for pass"},
	"cache_miss": {"value": 0, "flag": "a", "description": "Cache misses"},
	"backend_conn": {"value": 0, "flag": "a", "description": "Backend conn. success"},
	"LCK.sms.creat": {"type": "LCK", "ident": "sms", "value": 1, "flag": "a", "description": "Created locks"},
	"LCK.sms.destroy": {"type": "LCK", "ident": "sms", "value": 0, "flag": "a", "description": "Destroyed locks"},
	"VBE.default(127.0.0.1,,8080).vcls": {"type": "VBE", "ident": "default(127.0.0.1,,8080)", "value": 1, "flag": "i", "description": "VCL references"},
	"VBE.default(127.0.0.1,,8080).happy": {"type": "VBE", "ident": "default(127.0.0.1,,8080)", "value": 0, "flag": "b", "description": "Happy health probes"}
}`)}
	listResults := []int{7, 11}

	for i, json_ := range jsons {
		var (
			exporter varnishExporter
			err      error
		)
		if exporter.metrics, err = exporter.parseMetrics(bytes.NewBuffer(json_)); err != nil {
			t.Error(err.Error())
			continue
		}
		if len(exporter.metrics) != listResults[i] {
			t.Errorf("Found %d metrics, expected %d", len(exporter.metrics), listResults[i])
			continue
		}
		for _, m := range exporter.metrics {
			if m.Name == "" || m.Description == "" {
				t.Errorf("Failed to parse metric name/desc: %#v", m)
			}
		}
		if !t.Failed() {
			t.Logf("varnishstat -j: %d OK with %d metrics", i, len(exporter.metrics))
		}
	}

	// @todo The -l option is no longer used in the actual app, remove these tests
	// when that code is removed completely.
	lists := [][]byte{
		// varnish 4.x
		[]byte(`
Varnishstat -f option fields:
Field name                     Description
----------                     -----------
MAIN.uptime                    Child process uptime
MAIN.sess_conn                 Sessions accepted
MAIN.sess_drop                 Sessions dropped
MAIN.sess_fail                 Session accept failures
MAIN.client_req_400            Client requests received, subject to 400 errors
MAIN.client_req_417            Client requests received, subject to 417 errors
MAIN.client_req                Good client requests received
MAIN.cache_hit                 Cache hits
MAIN.cache_hitpass             Cache hits for pass`),
		// varnish 3.x
		[]byte(`
Varnishstat -f option fields:
Field name                     Description
----------                     -----------
client_conn                    Client connections accepted
client_drop                    Connection dropped, no sess/wrk
client_req                     Client requests received
cache_hit                      Cache hits
cache_hitpass                  Cache hits for pass
cache_miss                     Cache misses
backend_conn                   Backend conn. success
backend_unhealthy              Backend conn. not attempted`),
	}
	listResults = []int{9, 8}

	for i, list := range lists {
		var (
			exporter varnishExporter
			err      error
		)
		if exporter.metrics, err = exporter.parseMetricsList(bytes.NewBuffer(list)); err != nil {
			t.Error(err.Error())
			continue
		}
		if len(exporter.metrics) != listResults[i] {
			t.Errorf("Found %d metrics, expected %d", len(exporter.metrics), listResults[i])
			continue
		}
		for _, m := range exporter.metrics {
			if m.Name == "" || m.Description == "" {
				t.Errorf("Failed to parse metric name/desc: %#v", m)
			}
		}
		if !t.Failed() {
			t.Logf("varnishstat -l: %d OK with %d metrics", i, len(exporter.metrics))
		}
	}
}
