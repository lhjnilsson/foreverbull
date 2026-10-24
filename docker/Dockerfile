FROM node:26-alpine AS web

WORKDIR /web

COPY web/package.json web/package-lock.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

FROM golang:1.27.3-alpine

WORKDIR /app

COPY . /app
# go:embed reads internal/web/page at compile time. The stub is replaced by the Vite build.
COPY --from=web /web/dist /app/internal/web/page

RUN echo "" > .env

RUN go build -o /foreverbull cmd/server/main.go

EXPOSE 50055
EXPOSE 8080

ARG TARGETARCH
RUN wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.34/grpc_health_probe-linux-${TARGETARCH}
RUN chmod +x /bin/grpc_health_probe
HEALTHCHECK --interval=5s --timeout=3s CMD grpc_health_probe -addr=:50055

# gRPC, HTTP, and the backtest range are all above 1024. /tmp holds ETF snapshots.
RUN addgroup -S -g 10001 foreverbull \
 && adduser -S -D -u 10001 -G foreverbull -h /home/foreverbull -s /sbin/nologin foreverbull \
 && chown foreverbull:foreverbull /foreverbull /bin/grpc_health_probe \
 && chmod 1777 /tmp
USER foreverbull

CMD [ "/foreverbull" ]
