# K8s Pod Watcher
---
A lightweight **Kubernetes** Watcher made with **Go**, easily deploys to your cluster, and monitors for pod crashes, and immediately sends notifications to your Telegram. Resource-efficient alternative to a full Prometheus + Alertmanager setup for simple alerting.

![License](https://img.shields.io/badge/License-MIT-blue.svg)
[![Main Pipeline](https://github.com/Zapi-web/k8s-pod-watcher/actions/workflows/main.yaml/badge.svg?branch=main)](https://github.com/Zapi-web/k8s-pod-watcher/actions/workflows/main.yaml)

## Architecture
* **Go** - Core project language, all logic with `client-go`
* **Helm** - Flexible Kubernetes manifests for seamless, customizable deployment
* **Github Actions** - CI/CD pipeline that automatically pushes images/charts to the registry
* **GHCR.io** - Container/Chart registry for easy pull

## Features
* - **Structured logging:** All application logs are processed with `slog` in JSON format.
* - **Lightweight:** Highly resource-efficient, consuming significantly less RAM and CPU than production standards like Alertmanager.
* - **Anti-Spam Protection** Blocks duplicate notifications until container will restart again.
* - **Safety** Built on a distroless Docker base image combined with a strict `securityContext` to minimize the container's attack surface.
* - **Prometheus-ready** Exposes native metrics out of the box, allowing you to easily plug it into an existing monitoring stack if needed.

## Configuration

The application is configured dynamically using your Helm `values.yaml`.

| Value | Description | Default |
| :--- | :--- | :--- |
| `telegram.token` | Your Telegram Bot Father access token | `""` (Required) |
| `telegram.chatID` | Your target Telegram channel/chat ID | `""` (Required) |
| `logLevel` | Application log verbosity (`debug`, `info`, `warn`, `error`) | `"info"` |
| `metrics.enabled` | Enable/disable the metrics collection server | `false` |
| `metrics.port` | Network port for scraping and application health probes | `8080` |
| `metrics.path` | URL path path where metrics are exposed | `"/metrics"` |

## How To Install
### 1. Add your credentials to `values.yaml`
Create or modify your local `values.yaml` to include your Telegram authentication tokens:

```yaml
telegram:
  token: "YOUR_TELEGRAM_BOT_TOKEN"
  chatID: "YOUR_TELEGRAM_CHAT_ID"

metrics:
  enabled: true # Optional: turns on prometheus tracking & probes
```
### 2. Install the Chart via OCI Registry
Deploy the watcher directly into your cluster
```bash
helm install k8s-pod-watcher oci://ghcr.io/zapi-web/charts/k8s-pod-watcher \
  --version latest \
  -f values.yaml \
  --create-namespace \
  -n k8s-pod-watcher
```

## Endpoint
* GET /metrics; Emposes prometheus metrics
* GET /health; Returns 200 OK for liveness and readiness probes.