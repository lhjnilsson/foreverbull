FROM lhjnilsson/zipline_base:latest

COPY foreverbull_zipline/src /app

ARG FB_WHEEL=dist/foreverbull-0.0.1-py3-none-any.whl
COPY ${FB_WHEEL} /foreverbull-0.0.1-py3-none-any.whl
RUN pip install /foreverbull-0.0.1-py3-none-any.whl
RUN pip install grpcio-health-checking==1.66.1

RUN export PYTHONPATH="${PYTHONPATH}:/app/src"
WORKDIR /app

RUN wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.34/grpc_health_probe-linux-amd64
RUN chmod +x /bin/grpc_health_probe
HEALTHCHECK --interval=5s --timeout=3s CMD grpc_health_probe -addr=:50055

CMD ["python", "-m", "foreverbull_zipline"]
