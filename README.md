# httpserver

A zero-dependency Go CLI for instantly serving local static files over HTTP—built for frontend previews and local development without exposing your machine on the LAN or leaking dotfiles.

[![CI](https://github.com/ghchinoy/httpserver/actions/workflows/ci.yml/badge.svg)](https://github.com/ghchinoy/httpserver/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/ghchinoy/httpserver)](https://github.com/ghchinoy/httpserver/releases/latest)
[![Go Version](https://img.shields.io/github/go-mod/go-version/ghchinoy/httpserver)](https://go.dev/)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

## Contents

- [Installation](#installation)
- [Usage](#usage)
- [Security Defaults](#security-defaults)
- [Local Development](#local-development)
- [Publishing](#publishing)
- [Troubleshooting](#troubleshooting)
- [Contributing](#contributing)
- [License](#license)

## Installation

### Homebrew (macOS & Linux)

```bash
brew install ghchinoy/tap/httpserver
```

### Go

```bash
go install github.com/ghchinoy/httpserver@latest
```

Ensure your Go bin directory (typically `~/go/bin`) is in your `PATH`. Prebuilt archives for macOS, Linux, and Windows are also available on the [GitHub Releases](https://github.com/ghchinoy/httpserver/releases) page.

## Usage

Serve the current directory on loopback (`http://127.0.0.1:8085`):

```bash
httpserver
```

Example output:

```text
time=2026-11-21T22:40:00.000-07:00 level=INFO msg="server listening" url=http://127.0.0.1:8085 addr=127.0.0.1:8085 dir=/path/to/site cors="" dotfiles=false dir_listing=true
time=2026-11-21T22:40:03.112-07:00 level=INFO msg="http request" method=GET path=/ status=200 bytes=312 duration=145µs remote=127.0.0.1:54321
```

### Common Examples

```bash
# Serve a build directory on port 9000
httpserver -port 9000 -web ./dist

# Enable CORS for a local frontend dev server
httpserver -cors-origin http://localhost:3000 -web ./public

# Enable wildcard CORS for quick local prototyping
httpserver -cors -web ./web

# Expose on the local network with directory listings disabled
httpserver -host 0.0.0.0 -port 8085 -no-dir-listing

# Pick an ephemeral port with machine-readable JSON logs (for scripts and AI agents)
httpserver -port 0 -log-format json -web ./web
```

### Flags

| Flag | Default | Description |
| :--- | :--- | :--- |
| `-host` | `127.0.0.1` | Bind host address (loopback only by default). |
| `-port` | `8085` | TCP port number (`0` selects an available ephemeral port). |
| `-web` | `.` | Directory of static files to serve. |
| `-cors` | `false` | Enable permissive CORS (`Access-Control-Allow-Origin: *`). Mutually exclusive with `-cors-origin`. |
| `-cors-origin` | `""` | Enable CORS for a single explicit origin (sets `Vary: Origin`). |
| `-dotfiles` | `false` | Allow serving hidden files and directories (leading `.`). |
| `-no-dir-listing` | `false` | Disable automatic directory listings (directories with `index.html` still serve normally). |
| `-log-format` | `text` | Structured log format (`text` or `json`). |
| `-quiet` | `false` | Suppress per-request access logs (warnings and errors still print). |
| `-version` | `false` | Print version information and exit. |

## Security Defaults

`httpserver` is hardened for local development out of the box:

- **Loopback only**: Binds to `127.0.0.1` by default so files are not exposed on shared networks unless you pass `-host 0.0.0.0`.
- **Symlink & traversal containment**: Uses Go's `os.OpenRoot` (`http.FileServerFS`) so symlinks and `..` paths cannot escape `-web`.
- **Dotfiles hidden**: Requests for paths containing dot-segments (`.env`, `.git/config`, `.DS_Store`) return `404 Not Found` unless `-dotfiles` is passed.
- **CORS opt-in**: Disabled by default to prevent arbitrary websites in your browser from reading local files via `fetch`.
- **Read-only methods**: Only `GET`, `HEAD`, and `OPTIONS` are accepted; other methods return `405 Method Not Allowed`.
- **Server timeouts & headers**: Enforces `ReadHeaderTimeout` (10s), `ReadTimeout` (30s), `WriteTimeout` (60s), `IdleTimeout` (120s), `MaxHeaderBytes` (1 MiB), and sets `X-Content-Type-Options: nosniff`, `X-Frame-Options: SAMEORIGIN`, and `Referrer-Policy: no-referrer`.

## Local Development

Prerequisites:

- [Go](https://go.dev/dl/) `1.27.4` or newer
- [`golangci-lint`](https://golangci-lint.run/) v2 (required for `make check`, optional for `go test`)
- No environment variables are required; the dev server listens on `127.0.0.1:8085` by default.

Clone and run against the sample `./web` directory:

```bash
git clone https://github.com/ghchinoy/httpserver.git
cd httpserver
go run . -port 8085 -web ./web
```

Run tests, linter, and vulnerability checks:

```bash
go test -race -cover ./...
make check
make vulncheck
```

## Publishing

Releases are automated with [release-please](https://github.com/googleapis/release-please) and [GoReleaser](https://goreleaser.com/):

1. Merge commits to `main` using [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:`, etc.).
2. [release-please.yml](.github/workflows/release-please.yml) opens and updates a Release PR that bumps the semantic version and updates [CHANGELOG.md](CHANGELOG.md).
3. Merging the Release PR pushes tag `vX.Y.Z`, triggering [release.yml](.github/workflows/release.yml) to build reproducible binaries, upload archives and `checksums.txt` to GitHub Releases, and update `Casks/httpserver.rb` in [ghchinoy/homebrew-tap](https://github.com/ghchinoy/homebrew-tap).

One-time maintainer setup to store the cross-repo fine-grained PAT (`Contents: Read and write` on `ghchinoy/httpserver` and `ghchinoy/homebrew-tap`; `Pull requests: Read and write` on `ghchinoy/httpserver`):

```bash
gh secret set RELEASE_TOKEN --repo ghchinoy/httpserver
```

To validate the GoReleaser config and test a local snapshot build before releasing:

```bash
goreleaser check
goreleaser release --snapshot --clean
```

## Troubleshooting

- **Port already in use (`address already in use`)**: Choose another port with `httpserver -port 8086` or let the OS assign a free port with `httpserver -port 0`.
- **Browser blocks requests with a CORS error**: CORS is disabled by default for safety. Pass `-cors-origin http://localhost:3000` (recommended) or `-cors` for wildcard `*`.
- **Requests to `.well-known/*` or hidden files return `404`**: Dotfiles are blocked by default. Pass `-dotfiles` only when serving directories that do not contain secrets like `.env` or `.git`.
- **Cannot reach the server from another device on the LAN**: `httpserver` binds to `127.0.0.1` by default. Pass `-host 0.0.0.0` explicitly to listen on all interfaces.

## Contributing

Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

Before submitting a PR:

- Run `make check` and `make vulncheck` and ensure all checks pass.
- Add or update table-driven tests for behavioral changes.
- Use [Conventional Commits](https://www.conventionalcommits.org/) so release automation stays accurate.

## License

Licensed under the Apache License 2.0. See [LICENSE](LICENSE) for details.
