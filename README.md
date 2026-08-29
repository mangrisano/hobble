# hobble

<div align="center">

[![CI](https://github.com/mangrisano/hobble/actions/workflows/ci.yml/badge.svg)](https://github.com/mangrisano/hobble/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/tag/mangrisano/hobble?sort=semver&label=release)](https://github.com/mangrisano/hobble/tags)
[![Downloads](https://img.shields.io/github/downloads/mangrisano/hobble/total?label=downloads)](https://github.com/mangrisano/hobble/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

</div>

A fault-injection reverse proxy: sits between a client and a real target
service, and lets you deliberately inject latency, error status codes, or
dropped connections, so you can test how your client handles a flaky
backend. Successor to [`gaze`](https://github.com/mangrisano/gaze), same
spirit: small, focused, single static binary.

```sh
hobble -t https://api.example.com -l :8080 \
  --latency 200ms-800ms \
  --status 500=0.1 \
  --drop 0.05
```

Point your client at `hobble` instead of the real target, and observe how
it handles latency, errors, and dropped connections.

## Install

### Go

```sh
go install github.com/mangrisano/hobble/cmd/hobble@latest
```

Or from a local clone:

```sh
go install ./cmd/hobble
```

The binary lands in `$(go env GOPATH)/bin`, which must be on your `PATH`.

### Pre-built binaries

No Go toolchain? Grab a ready-made binary for your platform (Linux, macOS
and Windows, `amd64`/`arm64`) from the [latest
release](https://github.com/mangrisano/hobble/releases/latest). For
example, on macOS (arm64):

```sh
VERSION=0.1.0
curl -sL "https://github.com/mangrisano/hobble/releases/download/v${VERSION}/hobble_${VERSION}_darwin_arm64.tar.gz" | tar xz
./hobble --help
```

## Usage

```
hobble [flags]
```

| Flag              | Repeatable | Default | Description                                                             |
| ----------------- | :--------: | ------- | ------------------------------------------------------------------------ |
| `-t, --target`    |     no     | —       | Target URL to proxy requests to (required).                              |
| `-l, --listen`    |     no     | `:8080` | Address to listen on.                                                    |
| `--latency`       |     no     | `0`     | Latency to inject: fixed (`200ms`) or range (`200ms-800ms`).             |
| `--status`        |    yes     | —       | Status code to inject with a probability, e.g. `500=0.1`.                |
| `--drop`          |     no     | `0`     | Probability of dropping the connection (no response at all).             |

## Examples

Inject a fixed 500ms latency on every request:

```sh
hobble -t https://api.example.com --latency 500ms
```

Return a 500 on 10% of requests, and a 503 on 5%:

```sh
hobble -t https://api.example.com --status 500=0.1 --status 503=0.05
```

Simulate a flaky network: random latency, occasional 5xx, occasional
dropped connection:

```sh
hobble -t https://api.example.com \
  --latency 200ms-800ms \
  --status 500=0.1 \
  --drop 0.05
```

## How it works

Each request first rolls against `--drop`, then against the configured
`--status` rules, then (if neither fires) gets delayed by `--latency`
before being forwarded to the real target via a standard
`httputil.ReverseProxy`. Drop, status injection, and latency are mutually
exclusive per request — they don't stack on top of each other.

## Project layout

```
cmd/hobble/       package main — cobra flag parsing and wiring
internal/proxy/   package proxy — rule parsing, reverse proxy, fault middleware
```

## Development

```sh
go build ./...
go test ./...
go vet ./...
```

Requires Go 1.27+.

## License

MIT — see [LICENSE](LICENSE).
