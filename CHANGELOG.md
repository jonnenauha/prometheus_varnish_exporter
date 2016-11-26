1.2
=====
* Fix VBE label inconsistencies by always having `backend` and `server` labels present. ([#5](https://github.com/jonnenauha/prometheus_varnish_exporter/issues/5) [#8](https://github.com/jonnenauha/prometheus_varnish_exporter/issues/8))
 * Resulted in varnish reporting lots of errors for a while after VCL reloads.
* Fix bugs in `backend` and `server` label value parsing from VBE ident. ([#5](https://github.com/jonnenauha/prometheus_varnish_exporter/issues/5) [#8](https://github.com/jonnenauha/prometheus_varnish_exporter/issues/8))
* Add travis-ci build and test integration. Also auto pushes cross compiled binaries to github releases on tags.

1.1
===
* `-web.health-path <path>` can be configured to return a 200 OK response, by default not enabled. [#6](https://github.com/jonnenauha/prometheus_varnish_exporter/pull/6)
* Start building releases with go 1.7.3

1.0
===
* First official release
* Start buildings releases with go 1.7.1
