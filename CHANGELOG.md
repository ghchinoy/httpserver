# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and [Common Changelog](https://common-changelog.org/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.1](https://github.com/ghchinoy/httpserver/compare/v0.1.0...v0.1.1) (2026-11-22)


### Documentation

* improve README per make-readme checklist ([2df88e2](https://github.com/ghchinoy/httpserver/commit/2df88e2b7d7c9a83c4bc27c308a17abf4fb4c299))

## [Unreleased]

## [0.1.0] - 2026-11-21

_Initial release with hardened security defaults, structured logging, and automated Homebrew distribution._

### Added

- Configurable bind host (`-host`), explicit CORS controls (`-cors`, `-cors-origin`), dotfile opt-in (`-dotfiles`), directory listing toggle (`-no-dir-listing`), structured log formatting (`-log-format text|json`), quiet mode (`-quiet`), and version reporting (`-version`).
- Ephemeral port support (`-port 0`) with structured startup output reporting the bound URL.
- Graceful shutdown handling for `SIGINT` and `SIGTERM` with a bounded drain timeout.
- Comprehensive table-driven unit and security tests, `golangci-lint` configuration, CI workflow, `release-please`, and GoReleaser Homebrew tap publishing.

### Changed

- **Breaking:** Bind to loopback (`127.0.0.1`) by default instead of all interfaces (`0.0.0.0`); pass `-host 0.0.0.0` to expose files on the local network.
- **Breaking:** Disable permissive wildcard CORS by default to prevent cross-site local file reading; pass `-cors` or `-cors-origin <origin>` when CORS headers are needed.
- **Breaking:** Restrict HTTP methods strictly to `GET`, `HEAD`, and `OPTIONS` (returning `405 Method Not Allowed` for write methods such as `POST`).
- Migrated logging to structured `log/slog` with status code and byte count tracking.
- Replaced vendored Bootstrap v3.1.1 demo stylesheet in `web/` with minimal modern CSS.

### Security

- Upgraded Go toolchain baseline to `go 1.27.4`, resolving 7 reachable standard-library HTTP/1 and HTTP/2 vulnerabilities (`GO-2026-6603` through `GO-2026-6859`).
- Scoped file serving with `os.OpenRoot` and `http.FileServerFS` to prevent path traversal and symlink escapes outside `-web`.
- Blocked hidden dotfiles and dot-directories (such as `.env` and `.git`) by default unless `-dotfiles` is explicitly passed.
- Added HTTP server timeouts (`ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`) and `MaxHeaderBytes` limits to mitigate Slowloris and resource exhaustion.
- Added default security response headers (`X-Content-Type-Options: nosniff`, `X-Frame-Options: SAMEORIGIN`, `Referrer-Policy: no-referrer`) and CRLF header-injection validation on `-cors-origin`.

[Unreleased]: https://github.com/ghchinoy/httpserver/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/ghchinoy/httpserver/releases/tag/v0.1.0
