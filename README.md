# shm-agent

A lightweight, autonomous agent that collects metrics by parsing log files and sends them to a [SHM (Self-Hosted Metrics)](https://github.com/btouchard/shm) server.

## Features

- **Lightweight** — Single binary (~8MB), minimal dependencies
- **Log Tailing** — Continuous monitoring with log rotation support
- **Multiple Formats** — Parse JSON and regex-based log formats
- **Flexible Metrics** — Counter, gauge, sum, and set (cardinality) types
- **Powerful Matching** — Filter lines using equals, in, regex, or contains
- **Privacy-First** — Ed25519 signed requests, no PII collected by default
- **Dry-Run Mode** — Test configurations without sending data
- **Signal Support** — SIGUSR1 dumps metrics, graceful shutdown on SIGTERM

## Installation

### From Source

```bash
git clone https://github.com/btouchard/shm-agent.git
cd shm-agent
go build -ldflags="-s -w" -o shm-agent ./cmd/shm-agent
```

### Binary Releases

Download pre-built binaries from the [Releases](https://github.com/btouchard/shm-agent/releases) page.

## Quick Start

### 1. Create a Configuration File

```yaml
# /etc/shm-agent/config.yaml
server_url: https://shm.example.com
app_name: my-app
app_version: "1.0.0"
environment: production
interval: 60s

sources:
  - path: /var/log/nginx/access.log
    format: regex
    pattern: '^(?P<ip>\S+) \S+ \S+ \[(?P<time>[^\]]+)\] "(?P<method>\S+) (?P<path>\S+) [^"]*" (?P<status>\d+) (?P<bytes>\d+)'

    metrics:
      - name: http_requests
        type: counter

      - name: http_5xx
        type: counter
        match:
          field: status
          regex: "^5\\d{2}$"

      - name: bytes_served
        type: sum
        extract:
          field: bytes

      - name: unique_ips
        type: set
        extract:
          field: ip
```

### 2. Test Your Configuration

```bash
shm-agent test --config /etc/shm-agent/config.yaml /var/log/nginx/access.log
```

### 3. Run the Agent

```bash
# Production mode
shm-agent --config /etc/shm-agent/config.yaml

# Dry-run mode (prints metrics without sending)
shm-agent --config /etc/shm-agent/config.yaml --dry-run

# With custom interval
shm-agent --config /etc/shm-agent/config.yaml --dry-run --interval 5s
```

## Configuration Reference

### Global Options

| Field | Description | Default |
|-------|-------------|---------|
| `server_url` | SHM server URL | *required* |
| `app_name` | Application identifier | *required* |
| `app_version` | Application version | *required* |
| `environment` | Deployment environment | `production` |
| `interval` | Snapshot send interval | `60s` |
| `identity_file` | Path to identity JSON file | `./shm_identity.json` |

### Source Configuration

#### JSON Format

```yaml
sources:
  - path: /var/log/app/app.log
    format: json

    metrics:
      - name: requests_processed
        type: counter
        match:
          field: event
          equals: "request_processed"

      - name: active_sessions
        type: gauge
        extract:
          field: metrics.sessions.active  # Nested field access
```

#### Regex Format

```yaml
sources:
  - path: /var/log/nginx/access.log
    format: regex
    pattern: '^(?P<ip>\S+) .* "(?P<method>\S+) (?P<path>\S+) [^"]*" (?P<status>\d+) (?P<bytes>\d+)'

    metrics:
      - name: http_requests
        type: counter
```

### Metric Types

| Type | Behavior | Reset After Snapshot |
|------|----------|:--------------------:|
| `counter` | Increments by 1 for each matching line | Yes |
| `gauge` | Stores the last extracted value | No |
| `sum` | Sums all extracted numeric values | Yes |
| `set` | Counts unique values (cardinality) | Yes |

### Matching Conditions

| Condition | Description | Example |
|-----------|-------------|---------|
| `equals` | Exact string match | `equals: "error"` |
| `in` | Value in list | `in: ["error", "fatal"]` |
| `regex` | Regular expression match | `regex: "^5\\d{2}$"` |
| `contains` | Substring match | `contains: "timeout"` |

### Field Extraction

**JSON logs** — Use dot notation for nested fields:
```yaml
extract:
  field: response.body.bytes
```

**Regex logs** — Use named capture groups:
```yaml
pattern: '(?P<status>\d+) (?P<bytes>\d+)'
```

## CLI Reference

```
Usage: shm-agent --config=STRING <command> [flags]

Commands:
  run      Run the agent (default)
  test     Test configuration with a log file

Flags:
  -c, --config=STRING        Path to configuration file (required)
      --dry-run              Print metrics without sending to server
      --interval=DURATION    Override snapshot interval
  -v, --verbose              Increase verbosity (-v, -vv, -vvv)
  -h, --help                 Show help
```

### Examples

```bash
# Run with verbose output
shm-agent --config config.yaml -vv

# Test with line limit
shm-agent test --config config.yaml --lines 1000 /var/log/app.log

# Dry-run with short interval for debugging
shm-agent --config config.yaml --dry-run --interval 5s
```

## Signals

| Signal | Behavior |
|--------|----------|
| `SIGUSR1` | Dump current metrics to stdout (without reset) |
| `SIGTERM` | Graceful shutdown |
| `SIGINT` | Graceful shutdown |

```bash
# Dump current metrics
kill -USR1 $(pidof shm-agent)
```

## Example Configurations

### Nginx Access Logs

```yaml
server_url: https://shm.example.com
app_name: nginx
app_version: "1.24.0"
interval: 60s

sources:
  - path: /var/log/nginx/access.log
    format: regex
    pattern: '^(?P<ip>\S+) \S+ \S+ \[(?P<time>[^\]]+)\] "(?P<method>\S+) (?P<path>\S+) (?P<proto>[^"]*)" (?P<status>\d+) (?P<bytes>\d+)'

    metrics:
      - name: requests_total
        type: counter

      - name: requests_2xx
        type: counter
        match:
          field: status
          regex: "^2\\d{2}$"

      - name: requests_4xx
        type: counter
        match:
          field: status
          regex: "^4\\d{2}$"

      - name: requests_5xx
        type: counter
        match:
          field: status
          regex: "^5\\d{2}$"

      - name: bytes_sent
        type: sum
        extract:
          field: bytes

      - name: unique_visitors
        type: set
        extract:
          field: ip
```

### Traefik JSON Logs

```yaml
server_url: https://shm.example.com
app_name: traefik
app_version: "2.10.0"
interval: 60s

sources:
  - path: /var/log/traefik/access.json
    format: json

    metrics:
      - name: requests_total
        type: counter

      - name: requests_success
        type: counter
        match:
          field: OriginStatus
          regex: "^2\\d{2}$"

      - name: requests_error
        type: counter
        match:
          field: OriginStatus
          regex: "^[45]\\d{2}$"

      - name: latency_total_ns
        type: sum
        extract:
          field: Duration

      - name: unique_clients
        type: set
        extract:
          field: ClientHost
```

### Application JSON Logs

```yaml
server_url: https://shm.example.com
app_name: my-api
app_version: "2.1.0"
interval: 60s

sources:
  - path: /var/log/myapp/app.log
    format: json

    metrics:
      - name: requests_processed
        type: counter
        match:
          field: event
          equals: "request_completed"

      - name: errors_total
        type: counter
        match:
          field: level
          in: ["error", "fatal"]

      - name: active_connections
        type: gauge
        extract:
          field: metrics.connections

      - name: unique_users
        type: set
        extract:
          field: user_id

      - name: response_time_ms
        type: sum
        extract:
          field: duration_ms
```

## Systemd Service

```ini
# /etc/systemd/system/shm-agent.service
[Unit]
Description=SHM Agent - Log Metrics Collector
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/shm-agent --config /etc/shm-agent/config.yaml
Restart=always
RestartSec=5
User=shm-agent
Group=shm-agent

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadOnlyPaths=/
ReadWritePaths=/var/lib/shm-agent

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable shm-agent
sudo systemctl start shm-agent
```

## Architecture

```
shm-agent/
├── cmd/shm-agent/           # CLI entry point
└── agent/
    ├── config/              # YAML configuration parsing
    ├── parser/              # JSON and regex log parsers
    ├── matcher/             # Line matching logic
    ├── aggregator/          # Metric aggregation
    ├── tailer/              # File watching with rotation
    ├── identity/            # Ed25519 key management
    ├── sender/              # HTTP communication
    └── agent.go             # Main orchestration
```

## Development

### Prerequisites

- Go 1.22+

### Building

```bash
# Development build
go build -o shm-agent ./cmd/shm-agent

# Production build (smaller binary)
go build -ldflags="-s -w" -o shm-agent ./cmd/shm-agent
```

### Testing

```bash
# Run all tests
go test ./agent/... -v

# Run with race detector
go test ./agent/... -race

# Run specific package tests
go test ./agent/parser -v
```

## License

MIT License - see [LICENSE](LICENSE) for details.

## Related Projects

- [SHM Server](https://github.com/btouchard/shm) — Self-Hosted Metrics server
- [SHM Go SDK](https://github.com/btouchard/shm/tree/main/sdk/golang) — Go SDK for applications
- [SHM Node.js SDK](https://github.com/btouchard/shm/tree/main/sdk/nodejs) — Node.js SDK for applications
