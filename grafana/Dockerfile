# First stage: Build the Go binary
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# Copy Go module files
COPY . .
RUN go mod download

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o foreverbull_linux_arm64 .

# Build frontend
FROM node:18 AS js-builder
WORKDIR /app
COPY package.json .
COPY tsconfig.json .
COPY src/ src/
COPY README.md .
RUN npm install
RUN npm run build

# Second stage: Setup Grafana with the plugin
FROM grafana/grafana-oss:latest

# Create plugin directory
RUN mkdir -p /var/lib/grafana/plugins/foreverbull

# Copy binary from builder
COPY --from=builder /app/foreverbull_linux_arm64 /var/lib/grafana/plugins/foreverbull/
COPY --from=js-builder /app/dist /var/lib/grafana/plugins/foreverbull/
COPY src/plugin.json /var/lib/grafana/plugins/foreverbull/


# Set permissions for Grafana user (uid 472)
#RUN chown -R 472:472 /var/lib/grafana/plugins/foreverbull \
#    && chmod +x /var/lib/grafana/plugins/foreverbull/foreverbull

# Enable unsigned plugins
ENV GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS=foreverbull
ENV GF_AUTH_ANONYMOUS_ENABLED=true
ENV GF_AUTH_DISABLE_LOGIN_FORM=true
ENV GF_AUTH_ANONYMOUS_ORG_ROLE=Admin
ENV GF_AUTH_BASIC_ENABLED=false
ENV GF_AUTH_DISABLE_SIGNOUT_MENU=true

# Expose Grafana port
EXPOSE 3000
