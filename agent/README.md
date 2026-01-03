# shm-agent

A lightweight, autonomous agent that collects metrics by parsing log files and sends them to an SHM (Self-Hosted Metrics) server.

## Features

- **Log Tailing**: Continuously monitors log files with rotation support
- **Multiple Formats**: Parse JSON and regex-based log formats
- **Metric Types**:
  - `counter`: Increment on each matching line
  - `gauge`: Track the last extracted value
  - `sum`: Sum extracted numeric values
  - `set`: Count unique values (cardinality)
- **Flexible Matching**: Filter lines using equals, in, regex, or contains conditions
- **Dry-Run Mode**: Test configurations without sending data
- **Signal Support**: SIGUSR1 dumps current metrics without reset

## Installation

```bash
go build -o shm-agent ./cmd/shm-agent
```

## Usage

### Run the Agent

```bash
# Normal mode
shm-agent --config /etc/shm-agent/config.yaml

# Dry-run mode (print metrics, don't send)
shm-agent --config config.yaml --dry-run

# With custom interval
shm-agent --config config.yaml --dry-run --interval 5s

# Verbose output
shm-agent --config config.yaml -v      # Errors
shm-agent --config config.yaml -vv     # + Matches
shm-agent --config config.yaml -vvv    # + All lines
```

### Test a Configuration

```bash
# Process a log file with your config
shm-agent test --config config.yaml /var/log/nginx/access.log

# Limit lines processed
shm-agent test --config config.yaml --lines 100 /var/log/nginx/access.log
```

### Dump Metrics (SIGUSR1)

```bash
kill -USR1 $(pidof shm-agent)
```

## Configuration

```yaml
server_url: https://shm.example.com
identity_file: /var/lib/shm-agent/identity.json  # Optional
app_name: my-app
app_version: "1.0.0"
environment: production
interval: 60s

sources:
  # JSON log source
  - path: /var/log/myapp/app.log
    format: json
    metrics:
      - name: requests_count
        type: counter
        match:
          field: event
          equals: "request_processed"

      - name: errors_count
        type: counter
        match:
          field: level
          in: ["error", "fatal"]

      - name: active_sessions
        type: gauge
        extract:
          field: metrics.active_sessions

      - name: unique_users
        type: set
        extract:
          field: user_id

  # Regex log source (nginx)
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
```

## Matching Conditions

| Condition | Description | Example |
|-----------|-------------|---------|
| `equals` | Exact string match | `equals: "error"` |
| `in` | Value in list | `in: ["error", "fatal"]` |
| `regex` | Regular expression | `regex: "^5\\d{2}$"` |
| `contains` | Substring match | `contains: "error"` |

## Metric Types

| Type | Behavior | Reset After Snapshot |
|------|----------|---------------------|
| `counter` | +1 for each match | Yes |
| `gauge` | Last extracted value | No |
| `sum` | Sum of extracted values | Yes |
| `set` | Count of unique values | Yes |

## Field Extraction

For JSON logs, use dot notation for nested fields:
```yaml
extract:
  field: response.body.bytes
```

For regex logs, use named capture groups:
```yaml
pattern: '(?P<status>\d+) (?P<bytes>\d+)'
```

## Architecture

```
agent/
├── config/      # YAML configuration parsing
├── parser/      # JSON and regex log parsers
├── matcher/     # Line matching logic
├── aggregator/  # Metric aggregation
├── tailer/      # File watching with rotation
├── sender/      # HTTP communication
├── identity/    # Ed25519 key management
└── agent.go     # Main orchestration
```

## Testing

```bash
# Run all tests
go test ./agent/... -v

# Run integration tests
go test ./agent -run Integration -v
```

## License

MIT
