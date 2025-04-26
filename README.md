foreverbull
===================

Event driven backtesting and trading platform designed for continuous integration and delivery of algorithmic trading strategies.

Enabled by the [Zipline](https://github.com/stefan-jansen/zipline-reloaded) open source framework.

Currently support [Alpaca](https://alpaca.markets/) for market data and live trading.

# Getting Started

## Requirements

- Docker
- Python 3.10+
- Cookiecutter

## Quickstart

```bash
cookiecutter https://github.com/lhjnilsson/foreverbull.git
```

```bash
fbenv create
```

```bash
fbull backtest run DEMO
```

# Architecture

```mermaid
C4Container
title Architecture Overview

System_Ext(clientApp, "Client Application", "External client that connects to the Main Service")

System_Boundary(boundary, "Service Architecture") {
    System_Boundary(Application, "Application") {
        Container(mainService, "Main Service", "Go/Java/etc.", "Core service that accepts gRPC connections and orchestrates workflows")
        Container(grafana, "Grafana", "Monitoring", "Dashboard and analytics platform")
        Container(zipline, "Zipline", "Object Storage", "S3-compatible storage for files and binary data")
    }
    System_Boundary(Storage, "Storage") {
        ContainerDb(postgres, "PostgreSQL", "Database", "Stores persistent application data")
        ContainerQueue(nats, "NATS", "Message Queue", "Handles pub/sub messaging between services")
        Container(minio, "MinIO", "Object Storage", "S3-compatible storage for files and binary data")
    }
}

Rel(clientApp, mainService, "Makes requests to", "gRPC")
Rel(mainService, postgres, "Reads from and writes to", "SQL/TCP")
Rel(mainService, nats, "Publishes and subscribes to", "TCP")
Rel(mainService, minio, "Stores and retrieves objects", "HTTP/S3 API")
Rel(mainService, zipline, "Stores and retrieves objects", "HTTP/S3 API")
```

# Development Setup
