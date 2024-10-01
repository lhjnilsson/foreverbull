FROM golang:1.22-alpine

WORKDIR /app

COPY . /app

RUN echo "" > .env

RUN go build -o /foreverbull cmd/server/main.go

EXPOSE 50055

RUN wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.34/grpc_health_probe-linux-amd64
RUN chmod +x /bin/grpc_health_probe
HEALTHCHECK --interval=5s --timeout=3s CMD grpc_health_probe -addr=:50055

CMD [ "/foreverbull" ]
