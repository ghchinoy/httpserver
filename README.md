# Simple HTTP Server

A lightweight, zero-dependency CLI utility written in Go to instantly serve static files over HTTP with built-in CORS support and request logging.

[![Go Version](https://img.shields.io/github/go-mod/go-version/ghchinoy/httpserver)](https://golang.org)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

## Installation

Install the latest compiled binary using Go:

```bash
go install github.com/ghchinoy/httpserver@latest
```

Ensure your Go bin directory (typically `~/go/bin`) is in your system's `PATH`.

## Usage

To serve files from your current directory on the default port `8085`:

```bash
httpserver
```

### Options

* **`-port`**: Specify a custom port number (default: `8085`)
* **`-web`**: Path to the directory to serve (default: `.`)

Example with custom parameters:

```bash
httpserver -port 9000 -web ./dist
```

## Development

To set up the server locally for contributions:

1. Clone the repository:
   ```bash
   git clone https://github.com/ghchinoy/httpserver.git
   cd httpserver
   ```
2. Initialize and run without installing:
   ```bash
   go run httpserver.go -port 8085 -web ./web
   ```

## Contributing

Pull requests are welcome! For major architectural changes or feature additions, please open an issue first to discuss what you'd like to change.

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.
