FROM lhjnilsson/zipline_base:latest

# pip and the probe install need root. A rebuilt base already drops to foreverbull.
USER root

COPY zipline/src /app

ARG FB_WHEEL=dist/foreverbull-0.0.0-py3-none-any.whl
COPY ${FB_WHEEL} /foreverbull-0.0.0-py3-none-any.whl
RUN pip install /foreverbull-0.0.0-py3-none-any.whl
RUN pip install grpcio-health-checking==1.71.0

RUN export PYTHONPATH="${PYTHONPATH}:/app/src"
WORKDIR /app

ARG TARGETARCH=amd64
RUN wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.34/grpc_health_probe-linux-${TARGETARCH}
RUN chmod +x /bin/grpc_health_probe
# The published base may predate this user. Create it only when it is missing.
RUN if ! id -u foreverbull >/dev/null 2>&1; then \
      groupadd --gid 10001 foreverbull \
      && useradd --uid 10001 --gid 10001 --create-home --home-dir /home/foreverbull --shell /usr/sbin/nologin foreverbull; \
    fi \
 && chown -R foreverbull:foreverbull /app /bin/grpc_health_probe \
 && chmod 1777 /tmp
USER foreverbull
HEALTHCHECK --interval=5s --timeout=3s CMD grpc_health_probe -addr=:50055

CMD ["python", "-m", "foreverbull_zipline"]
