# httpserver

A zero-dependency Go CLI for serving static files over HTTP with secure defaults, optional CORS, and structured logging.

[![CI](https://github.com/ghchinoy/httpserver/actions/workflows/ci.yml/badge.svg)](https://github.com/ghchinoy/httpserver/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/ghchinoy/httpserver)](https://go.dev/)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

## Installation

### Homebrew (macOS & Linux)

```bash
brew install ghchinoy/tap/httpserver
```

### Go

```bash
go install github.com/ghchinoy/httpserver@latest
```

Make sure your Go bin directory (typically `~/go/bin`) is in your `PATH`. Prebuilt binaries for macOS, Linux, and Windows are also attached to every [GitHub Release](https://github.com/ghchinoy/httpserver/releases).

## Usage

Serve the current directory on loopback (`http://127.0.0.1:8085`):

```bash
httpserver
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

# Pick an ephemeral port with machine-readable JSON logs (great for scripts and agents)
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
- **Server timeouts & headers**: Enforces `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`, `MaxHeaderBytes` (1 MiB), and sets `X-Content-Type-Options: nosniff`, `X-Frame-Options: SAMEORIGIN`, and `Referrer-Policy: no-referrer`.

## Local Development

Prerequisites:

- Go `1.27.4` or newer
- [`golangci-lint`](https://golangci-lint.run/) v2 (for `make check`)

Clone, run locally, and verify formatting, linting, tests, and vulnerabilities:

```bash
git clone https://github.com/ghchinoy/httpserver.git
cd httpserver
go run . -port 8085 -web ./web
```

Run the full quality gate:

```bash
make check
make vulncheck
```

## Releases

Releases are automated with [release-please](https://github.com/googleapis/release-please) and [GoReleaser](https://goreleaser.com/):

1. Merge commits to `main` using [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `chore:`, etc.).
2. `release-please` opens and updates a Release PR that bumps the semantic version and updates [CHANGELOG.md](CHANGELOG.md).
3. Merging the Release PR tags `vX.Y.Z`, which triggers [release.yml](.github/workflows/release.yml) to build reproducible binaries, upload archives + `checksums.txt` to GitHub Releases, and update `Casks/httpserver.rb` in [ghchinoy/homebrew-tap](https://github.com/ghchinoy/homebrew-tap).

> Maintainer setup: configure a repository secret named `RELEASE_TOKEN` (fine-grained PAT with `Contents: Read and write` on `ghchinoy/httpserver` and `ghchinoy/homebrew-tap`, plus `Pull requests: Read and write` on `ghchinoy/httpserver`).

## Contributing

Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

Before submitting a PR:

- Run `make check` and `make vulncheck` and ensure all checks pass.
- Add or update table-driven tests for behavioral changes.
- Use Conventional Commit messages so release notes stay accurate.

## License

Licensed under the Apache License 2.0. See [LICENSE](LICENSE) for details.
