FROM        ubuntu:trusty
MAINTAINER  Luke Swithenbank <swithenbank.luke@gmail.com>

ENV DEBIAN_FRONTEND noninteractive

# Install Varnish to make the exporter work as expected
RUN apt-get -qq update
RUN apt-get install -y apt-transport-https curl
RUN curl https://repo.varnish-cache.org/GPG-key.txt | apt-key add -
RUN echo "deb https://repo.varnish-cache.org/ubuntu/ trusty varnish-4.1" \
  | tee -a /etc/apt/sources.list.d/varnish-cache.list
RUN apt-get -qq update
RUN apt-get install -y varnish

COPY varnish_exporter /bin/varnish_exporter

EXPOSE      9131
ENTRYPOINT  [ "/bin/varnish_exporter" ]
