FROM        quay.io/prometheus/busybox:latest
MAINTAINER  Luke Swithenbank <swithenbank.luke@gmail.com>

COPY varnish_exporter /bin/varnish_exporter

EXPOSE      9131
ENTRYPOINT  [ "/bin/varnish_exporter" ]
