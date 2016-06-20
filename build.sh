#!/usr/bin/env bash
./gow get github.com/mitchellh/gox
./gow get
./build/bin/gox -output="build/bin/prometheus-varnish-exporter/{{.Dir}}_{{.OS}}_{{.Arch}}"