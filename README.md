# Varnish exporter for Prometheus

> Export Varnish statistics to Prometheus. Uses `varnishstat` on the host machine to scrape stats and exports them via HTTP for Prometheus.

Scrapes the `varnishstat -j` JSON output on each Prometheus collect and exposes all reported metrics. Metrics with multiple backends or varnish defined identifiers (e.g. `VBE.*.happy SMA.*.c_bytes LCK.*.creat`) and other metrics with similar structure (e.g. `MAIN.fetch_*`) are combined under a single metric name with distinguishable labels. Vanish naming conventions are preserved as much as possible to be familiar to Varnish users when building queries, while at the same time following Prometheus conventions like lower casing and using `_` separators.

Handles runtime Varnish changes like adding new backends via vlc reload. Removed backends are reported by `varnishstat` until Varnish is restarted.

Advanced users can use `-n -N`, they are passed to `varnishstat`.

Tested against Varnish 4.x and 3.x output. Missing category groupings in 3.x like `MAIN.` are detected and added automatically for label names to be consistent across versions, assuming of course that the Varnish project does not remove/change the stats.

I won't make any backwards compatibility promises at this point. Your built queries can break on new versions if metric names or labels are refined.

If you find bugs or have feature requests feel free to create issues or send PRs.

# Installing and running

```
go get github.com/jonnenauha/prometheus_varnish_exporter
$GOPATH/bin/prometheus_varnish_exporter -h
```

# Test mode

To test that `varnishstat` is found on the host machine and to preview all exported metrics run

```
$GOPATH/bin/prometheus_varnish_exporter -test
```
