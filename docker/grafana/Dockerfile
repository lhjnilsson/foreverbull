FROM golang:1.22 as loki

RUN git clone https://github.com/grafana/loki --branch v3.1.1 /loki
WORKDIR /loki
RUN make loki

FROM grafana/grafana-oss

USER root

COPY grafana.ini /etc/grafana/grafana.ini

COPY --from=loki /loki/cmd/loki/loki /usr/local/bin/loki

RUN mkdir /etc/loki
COPY loki-config.yaml /etc/loki/local-config.yaml

RUN mkdir /opt/loki

RUN apk update && apk add --no-cache supervisor

COPY supervisord.conf /etc/supervisord.conf
RUN mkdir -p /var/log/supervisord
RUN chmod 777 /var/log/supervisord

RUN grafana-cli plugins install yesoreyeram-infinity-datasource
COPY datasources/default.yaml /etc/grafana/provisioning/datasources/default.yaml

EXPOSE 3000
EXPOSE 3100

ENTRYPOINT ["/usr/bin/supervisord"]
CMD ["-c", "/etc/supervisord.conf"]
