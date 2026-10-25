FROM node:26-alpine AS web

WORKDIR /web

COPY web/package.json web/package-lock.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

FROM golang:1.27.3-alpine AS build

WORKDIR /app

COPY . /app
# go:embed reads internal/web/page at compile time. The stub is replaced by the Vite build.
COPY --from=web /web/dist /app/internal/web/page

RUN echo "" > .env

# Static binary so the runtime image needs no Go toolchain or C library from the builder.
RUN CGO_ENABLED=0 go build -o /foreverbull ./cmd/server

ARG TARGETARCH
RUN wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.34/grpc_health_probe-linux-${TARGETARCH} \
 && chmod +x /bin/grpc_health_probe

# Runtime only: binary, probe, and CA certs. Matches the builder's Alpine major.
FROM alpine:3.24

RUN apk add --no-cache ca-certificates \
 && addgroup -S -g 10001 foreverbull \
 && adduser -S -D -u 10001 -G foreverbull -h /home/foreverbull -s /sbin/nologin foreverbull \
 && chmod 1777 /tmp

COPY --from=build /foreverbull /foreverbull
COPY --from=build /bin/grpc_health_probe /bin/grpc_health_probe
RUN chown foreverbull:foreverbull /foreverbull /bin/grpc_health_probe

USER foreverbull

EXPOSE 50055
EXPOSE 8080

HEALTHCHECK --interval=5s --timeout=3s CMD grpc_health_probe -addr=:50055

CMD [ "/foreverbull" ]
