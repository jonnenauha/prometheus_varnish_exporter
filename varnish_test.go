package main

import (
	"bytes"
	"fmt"
	"testing"
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
		"varnish 1.0": &varnishVersion{
			Major: 1, Minor: 0, Patch: -1,
		},
	}
	for versionStr, test := range tests {
		var v varnishVersion
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
	StartParams.Raw = true

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
  "VBE.81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu1.happy": {
    "description": "Happy health probes",
    "type": "VBE", "ident": "81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu1", "flag": "b", "format": "b",
    "value": 18446744073709551615
  },
  "VBE.81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu1.bereq_hdrbytes": {
    "description": "Request header bytes",
    "type": "VBE", "ident": "81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu1", "flag": "c", "format": "B",
    "value": 213113278
  },
  "VBE.81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu1.bereq_bodybytes": {
    "description": "Request body bytes",
    "type": "VBE", "ident": "81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu1", "flag": "c", "format": "B",
    "value": 119721
  },
  "VBE.81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu1.beresp_hdrbytes": {
    "description": "Response header bytes",
    "type": "VBE", "ident": "81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu1", "flag": "c", "format": "B",
    "value": 110012037
  },
  "VBE.81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu1.beresp_bodybytes": {
    "description": "Response body bytes",
    "type": "VBE", "ident": "81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu1", "flag": "c", "format": "B",
    "value": 7927018558
  },
  "VBE.81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu1.pipe_hdrbytes": {
    "description": "Pipe request header bytes",
    "type": "VBE", "ident": "81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu1", "flag": "c", "format": "B",
    "value": 661
  },
  "VBE.81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu1.pipe_out": {
    "description": "Piped bytes to backend",
    "type": "VBE", "ident": "81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu1", "flag": "c", "format": "B",
    "value": 0
  },
  "VBE.81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu1.pipe_in": {
    "description": "Piped bytes from backend",
    "type": "VBE", "ident": "81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu1", "flag": "c", "format": "B",
    "value": 899
  },
  "VBE.81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu1.conn": {
    "description": "Concurrent connections to backend",
    "type": "VBE", "ident": "81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu1", "flag": "g", "format": "i",
    "value": 401853
  },
  "VBE.81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu1.req": {
    "description": "Backend requests sent",
    "type": "VBE", "ident": "81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu1", "flag": "c", "format": "i",
    "value": 401853
  },
  "VBE.81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu2.happy": {
    "description": "Happy health probes",
    "type": "VBE", "ident": "81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu2", "flag": "b", "format": "b",
    "value": 18446744073709551615
  },
  "VBE.81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu2.bereq_hdrbytes": {
    "description": "Request header bytes",
    "type": "VBE", "ident": "81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu2", "flag": "c", "format": "B",
    "value": 213118630
  },
  "VBE.81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu2.bereq_bodybytes": {
    "description": "Request body bytes",
    "type": "VBE", "ident": "81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu2", "flag": "c", "format": "B",
    "value": 236848
  },
  "VBE.81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu2.beresp_hdrbytes": {
    "description": "Response header bytes",
    "type": "VBE", "ident": "81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu2", "flag": "c", "format": "B",
    "value": 110017976
  },
  "VBE.81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu2.beresp_bodybytes": {
    "description": "Response body bytes",
    "type": "VBE", "ident": "81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu2", "flag": "c", "format": "B",
    "value": 7825794805
  },
  "VBE.81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu2.pipe_hdrbytes": {
    "description": "Pipe request header bytes",
    "type": "VBE", "ident": "81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu2", "flag": "c", "format": "B",
    "value": 743
  },
  "VBE.81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu2.pipe_out": {
    "description": "Piped bytes to backend",
    "type": "VBE", "ident": "81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu2", "flag": "c", "format": "B",
    "value": 0
  },
  "VBE.81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu2.pipe_in": {
    "description": "Piped bytes from backend",
    "type": "VBE", "ident": "81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu2", "flag": "c", "format": "B",
    "value": 1056
  },
  "VBE.81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu2.conn": {
    "description": "Concurrent connections to backend",
    "type": "VBE", "ident": "81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu2", "flag": "g", "format": "i",
    "value": 401855
  },
  "VBE.81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu2.req": {
    "description": "Backend requests sent",
    "type": "VBE", "ident": "81d82226-e891-458e-b7b8-13bdc0ccb1ee.eu2", "flag": "c", "format": "i",
    "value": 401855
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
	listResults := []int{25, 11}
	versions := []int{4, 3}

	for i, json_ := range jsons {
		fmt.Println("\n")
		var (
			exporter = NewVarnishExporter()
			importer = NewPrometheusExporter()
			err      error
		)
		Varnish.Version.Major = versions[i]

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
		if err = importer.exposeMetrics(exporter.metrics); err == nil {
			dumpMetrics(importer)
			fmt.Println(" ")
		} else {
			t.Errorf("Exposing metrics failed: %s", err.Error())
		}
		if !t.Failed() {
			t.Logf("varnishstat -j: %d OK with %d metrics", i, len(exporter.metrics))
		}
	}
}
