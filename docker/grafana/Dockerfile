FROM golang:1.22 as loki

RUN git clone https://github.com/grafana/loki --branch v3.1.1 /loki
WORKDIR /loki
RUN make loki

FROM grafana/grafana-oss

USER root

COPY --from=loki /loki/cmd/loki/loki /usr/local/bin/loki

RUN mkdir /etc/loki
COPY loki-config.yaml /etc/loki/local-config.yaml

RUN mkdir /opt/loki

RUN apk update && apk add --no-cache supervisor

COPY supervisord.conf /etc/supervisord.conf
RUN mkdir -p /var/log/supervisord


ENTRYPOINT ["/usr/bin/supervisord", "-c", "/etc/supervisord.conf"]
