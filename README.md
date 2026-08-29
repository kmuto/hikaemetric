# hikaemetric

- [日本語](README-ja.md)

A tool that receives OTLP metrics via HTTP/Protobuf and writes them to TSV files.

In addition to sending metrics from an OpenTelemetry Collector to your observability platform, you can fork them to this tool to keep local TSV files that are easy to work with in spreadsheets.

## Installation

Download a package for your OS and architecture from [Releases](https://github.com/kmuto/hikaemetric/releases).

- Linux: deb / rpm / zip (amd64, arm64)
- macOS: zip (arm64)
- Windows: zip (amd64)

## Usage

```
hikaemetric [options]
```

### Options

| Option | Default | Description |
|---|---|---|
| `-port` | 14318 | OTLP HTTP listen port |
| `-output` | `.` | Output directory for TSV files |
| `-resource-attrs` | (none) | Comma-separated resource attribute keys to include |
| `-attrs` | (none) | Comma-separated attribute keys to include |
| `-timezone` | system local | Timezone for timestamps and file rotation (e.g. `Asia/Tokyo`) |

### Example

```sh
hikaemetric \
  -port 14318 \
  -output /var/log/metrics \
  -timezone Asia/Tokyo \
  -resource-attrs host.name,service.version \
  -attrs method,status_code
```

## Output

### File naming

```
{namespace}-{service_name}-YYYYMMDD.tsv
```

If `service.namespace` is not set, the file is named `{service_name}-YYYYMMDD.tsv`. The date is based on the metric timestamp in the configured timezone.

Characters other than alphanumerics, `_`, and `-` are stripped from the file name.

### TSV format

The first line is a header.

```
timestamp	name	type	value	unit	resource.{key}...	{key}...
```

- `timestamp`: RFC 3339 format in the configured timezone
- `type`: `gauge` / `counter` / `histogram` / `exponential_histogram` / `summary`
- `value`: numeric value; JSON string for histogram types

Example with `-resource-attrs host.name -attrs method`:

```tsv
timestamp	name	type	value	unit	resource.host.name	method
2026-08-29T15:00:00+09:00	cpu.usage	gauge	82.3	%	host1	GET
```

Histogram `value` example:

```json
{"count":50,"sum":350.5,"bucket_counts":[5,20,15,10],"boundaries":[10,50,100]}
```

### File rotation

A new file is created when the date changes. If the process is restarted on the same day, rows are appended to the existing file without duplicating the header.

## Integration with OTel Collector

Add an `otlp_http` exporter in the Collector config to send metrics to both your platform and hikaemetric.

```yaml
exporters:
  otlp_http/platform:
    endpoint: https://your-platform.example.com
  otlp_http/hikae:
    endpoint: http://localhost:14318

service:
  pipelines:
    metrics:
      exporters: [otlp_http/platform, otlp_http/hikae]
```

## Development

```sh
go test ./...
go build -o hikaemetric .
```

### Sample sender

`sample/` contains a tool that sends random metrics of various types.

```sh
go run ./sample/
```

## License

© 2026 Kenshi Muto

MIT License ([LICENSE](LICENSE))
