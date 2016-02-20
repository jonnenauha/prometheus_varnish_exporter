# Varnish exporter for Prometheus

> Export Varnish statistics to Prometheus. Uses `varnishstat` on the host machine to scrape stats and exports them via HTTP/JSON for Prometheus.

Scrapes the `varnishstat -j` JSON output and exposes all available metrics. Metrics with multiple backends or varnish defined identifiers (e.g. `VBE.*.happy SMA.*.c_bytes LCK.*.creat`) and other metrics with similar structure (e.g. `MAIN.fetch_*`) are combined under a single metric name with distinguishable labels. Vanish naming conventions are preserved as much as possible to be familiar to Varnish users when building queries, while at the same time following Prometheus conventions like lower casing and using `_` separators.

Tested against Varnish 4.x and 3.x `varnishstat -j` output. Missing category groupings in 3.x like `MAIN.` are detected and added automatically for label names to be consistent across versions, assuming of course that the Varnish project does not remove/change the stats.

This was a one night project for me after I could not find an existing exporter. Keep that in mind if you want to take this to production. Limited testing was done to a fresh Varnish 3.0.5 on Ubuntu 14.04 VM. Executed test queries from Prometheus to verify it works and updates values, it does!

If you find bugs or have feature requests feel free to create issues or send PRs.

# Installing and running

```
go get github.com/jonnenauha/prometheus_varnish_exporter
$GOPATH/bin/prometheus_varnish_exporter -h
```

# Dryrun

To test that `varnishstat` is found on the host machine and to preview all exported metrics run

```
$GOPATH/bin/prometheus_varnish_exporter -test
```
