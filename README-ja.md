# hikaemetric

- [English](README.md)

OTLPメトリックをHTTP/Protobufで受け取り、TSVファイルに書き出すツールです。

OpenTelemetryコレクターから本来のオブザーバビリティプラットフォームに送る構成に加えて、このツールにも分岐して送ることで、スプレッドシートなどで手軽に集計できるTSVを手元に残せます。

## インストール

[Releases](https://github.com/kmuto/hikaemetric/releases) からOS・アーキテクチャに合ったパッケージをダウンロードしてください。

- Linux: deb / rpm / zip (amd64, arm64)
- macOS: zip (arm64)
- Windows: zip (amd64)

## 使い方

```
hikaemetric [options]
```

### オプション

| オプション | デフォルト | 説明 |
|---|---|---|
| `-port` | 14318 | OTLP HTTP受信ポート |
| `-output` | `.` | TSV出力ディレクトリ |
| `-resource-attrs` | (なし) | TSVに含めるリソース属性キー（カンマ区切り） |
| `-attrs` | (なし) | TSVに含める属性キー（カンマ区切り） |
| `-timezone` | システムローカル | タイムスタンプとファイルローテーションのタイムゾーン（例: `Asia/Tokyo`） |

### 例

```sh
hikaemetric \
  -port 14318 \
  -output /var/log/metrics \
  -timezone Asia/Tokyo \
  -resource-attrs host.name,service.version \
  -attrs method,status_code
```

## 出力

### ファイル名

```
{サービス名前空間}-{サービス名}-YYYYMMDD.tsv
```

`service.namespace` がない場合は `{サービス名}-YYYYMMDD.tsv` になります。日付はメトリックのタイムスタンプ基準で、指定タイムゾーンに従います。

ファイル名に使えない文字（英数字・`_`・`-` 以外）は除去されます。

### TSVフォーマット

先頭行がヘッダーです。

```
timestamp	name	type	value	unit	resource.{key}...	{key}...
```

- `timestamp`: RFC 3339形式（指定タイムゾーン）
- `type`: `gauge` / `counter` / `histogram` / `exponential_histogram` / `summary`
- `value`: 数値。ヒストグラム系はJSON文字列

例（`-resource-attrs host.name -attrs method` の場合）:

```tsv
timestamp	name	type	value	unit	resource.host.name	method
2026-08-29T15:00:00+09:00	cpu.usage	gauge	82.3	%	host1	GET
```

ヒストグラムの `value` 例:

```json
{"count":50,"sum":350.5,"bucket_counts":[5,20,15,10],"boundaries":[10,50,100]}
```

### ファイルローテーション

日付が変わると新しいファイルが作られます。同日中にプロセスを再起動した場合は既存ファイルに追記します（ヘッダーは重複しません）。

## OTelコレクターとの連携

コレクターの設定で `otlp_http` エクスポーターを追加し、既存の送信先と並列に送ります。

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

## 開発

```sh
go test ./...
go build -o hikaemetric .
```

### サンプル送信

`sample/` にさまざまなメトリック型をランダムに送るツールがあります。

```sh
go run ./sample/
```

## ライセンス

© 2026 Kenshi Muto

MIT License ([LICENSE](LICENSE))
