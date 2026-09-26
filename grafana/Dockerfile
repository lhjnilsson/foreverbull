# syntax=docker/dockerfile:1
# Plugin go.mod requires Go 1.24. `replace => ../..` needs the repo module
# two directories above the plugin. Grafana OSS stays 11.3.2.
# Build with: docker build -f Dockerfile --build-context foreverbull=.. .
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Layout matches replace github.com/lhjnilsson/foreverbull => ../..
WORKDIR /src/grafana/lhjnilsson-foreverbull-datasource

COPY --from=foreverbull go.mod go.sum /src/
COPY --from=foreverbull pkg /src/pkg

COPY lhjnilsson-foreverbull-datasource/go.mod lhjnilsson-foreverbull-datasource/go.sum ./
RUN go mod download

COPY lhjnilsson-foreverbull-datasource/ .
RUN go install github.com/magefile/mage@latest
RUN mage

# Build frontend
FROM node:22 AS js-builder
WORKDIR /app
COPY lhjnilsson-foreverbull-datasource/ .
RUN npm install
RUN npm run build

# Second stage: Setup Grafana with the plugin
FROM grafana/grafana-oss:11.3.2

# Create plugin directory
RUN mkdir -p /var/lib/grafana/plugins/lhjnilsson-foreverbull-datasource/

# Copy binary from builder
COPY --from=builder /src/grafana/lhjnilsson-foreverbull-datasource/dist/* /var/lib/grafana/plugins/lhjnilsson-foreverbull-datasource/
COPY --from=js-builder /app/dist/* /var/lib/grafana/plugins/lhjnilsson-foreverbull-datasource/
COPY lhjnilsson-foreverbull-datasource/provisioning/ /etc/grafana/provisioning

# Set permissions for Grafana user (uid 472)
#RUN chown -R 472:472 /var/lib/grafana/plugins/foreverbull \
#    && chmod +x /var/lib/grafana/plugins/foreverbull/foreverbull

# Enable unsigned plugins
ENV GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS=lhjnilsson-foreverbull-datasource
ENV GF_AUTH_ANONYMOUS_ENABLED=true
ENV GF_AUTH_DISABLE_LOGIN_FORM=true
ENV GF_AUTH_ANONYMOUS_ORG_ROLE=Admin
ENV GF_AUTH_BASIC_ENABLED=false
ENV GF_AUTH_DISABLE_SIGNOUT_MENU=true

# Expose Grafana port
EXPOSE 3000
