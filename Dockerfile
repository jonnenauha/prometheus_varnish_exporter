FROM golang:latest as gobuilder


ADD ./* /Workingdir
WORKDIR /Workingdir

ARG BuildPath=/go/bin/prometheus_varnish_exporter

RUN go build -o $BuildPath

FROM varnish

EXPOSE 9131
VOLUME /var/lib/varnish
ENTRYPOINT ["/usr/local/bin/prometheus_varnish_exporter"]
COPY --from=gobuilder /go/bin/prometheus_varnish_exporter /usr/local/bin
