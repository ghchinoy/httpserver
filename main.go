// Command httpserver serves static files from a local directory over HTTP
// with secure defaults, optional CORS headers, and structured request logging.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
)

// Build metadata injected at link time via -ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	os.Exit(runMain())
}

func runMain() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		if hint := errorHint(err); hint != "" {
			_, _ = fmt.Fprintf(os.Stderr, "hint: %s\n", hint)
		}
		return 1
	}
	return 0
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	cfg, showVersion, err := parseFlags(args, stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	if showVersion {
		_, err := fmt.Fprintln(stdout, formatVersion())
		return err
	}

	if err := validateConfig(cfg); err != nil {
		return err
	}

	logger := newLogger(stderr, cfg.logFormat, cfg.quiet)
	return serve(ctx, cfg, logger)
}

func parseFlags(args []string, output io.Writer) (config, bool, error) {
	var (
		cfg         config
		showVersion bool
	)

	fs := flag.NewFlagSet("httpserver", flag.ContinueOnError)
	fs.SetOutput(output)

	fs.StringVar(&cfg.host, "host", defaultHost, "bind host address (default loopback only)")
	fs.IntVar(&cfg.port, "port", defaultPort, "TCP port number (0 for an ephemeral port)")
	fs.StringVar(&cfg.webDir, "web", defaultWebDir, "directory of static files to serve")
	fs.BoolVar(&cfg.corsWildcard, "cors", false, "enable permissive CORS (Access-Control-Allow-Origin: *)")
	fs.StringVar(&cfg.corsOrigin, "cors-origin", "", "enable CORS for a specific origin (e.g. http://localhost:3000)")
	fs.BoolVar(&cfg.allowDotfiles, "dotfiles", false, "allow serving hidden files and directories (leading dot)")
	fs.BoolVar(&cfg.noDirListing, "no-dir-listing", false, "disable automatic directory listings")
	fs.StringVar(&cfg.logFormat, "log-format", logFormatText, "log output format: text or json")
	fs.BoolVar(&cfg.quiet, "quiet", false, "suppress per-request access logs")
	fs.BoolVar(&showVersion, "version", false, "print version information and exit")

	fs.Usage = func() {
		_, _ = fmt.Fprintf(output, "Usage: httpserver [flags]\n\n")
		_, _ = fmt.Fprintf(output, "Serve static files from a local directory over HTTP with secure defaults.\n\n")
		_, _ = fmt.Fprintf(output, "Flags:\n")
		fs.PrintDefaults()
		_, _ = fmt.Fprintf(output, "\nExamples:\n")
		_, _ = fmt.Fprintf(output, "  httpserver\n")
		_, _ = fmt.Fprintf(output, "  httpserver -port 9000 -web ./dist\n")
		_, _ = fmt.Fprintf(output, "  httpserver -cors-origin http://localhost:3000 -web ./public\n")
		_, _ = fmt.Fprintf(output, "  httpserver -host 0.0.0.0 -port 8085 -no-dir-listing\n")
		_, _ = fmt.Fprintf(output, "  httpserver -port 0 -log-format json -web ./web\n")
	}

	if err := fs.Parse(args); err != nil {
		return config{}, false, err
	}

	if fs.NArg() > 0 {
		return config{}, false, fmt.Errorf("%w: %q", errUnexpectedArgs, strings.Join(fs.Args(), " "))
	}

	return cfg, showVersion, nil
}

func formatVersion() string {
	ver := version
	if ver == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			ver = info.Main.Version
		}
	}
	return fmt.Sprintf("httpserver %s (commit %s, built %s)", ver, commit, date)
}

func newLogger(w io.Writer, format string, quiet bool) *slog.Logger {
	level := slog.LevelInfo
	if quiet {
		level = slog.LevelWarn
	}
	opts := &slog.HandlerOptions{Level: level}
	if format == logFormatJSON {
		return slog.New(slog.NewJSONHandler(w, opts))
	}
	return slog.New(slog.NewTextHandler(w, opts))
}

func errorHint(err error) string {
	switch {
	case errors.Is(err, errInvalidPort):
		return "choose a port between 0 and 65535, e.g. 'httpserver -port 8085'"
	case errors.Is(err, errWebDirNotFound):
		return "check that the directory exists, e.g. 'httpserver -web .'"
	case errors.Is(err, errWebDirNotDirectory):
		return "pass a directory path instead of a file, e.g. 'httpserver -web ./public'"
	case errors.Is(err, errConflictingCORSFlags):
		return "use either '-cors' or '-cors-origin <origin>', not both"
	case errors.Is(err, errInvalidCORSOrigin):
		return "pass a single-line origin without control characters, e.g. 'http://localhost:3000'"
	case errors.Is(err, errInvalidLogFormat):
		return "use '-log-format text' or '-log-format json'"
	case errors.Is(err, errUnexpectedArgs):
		return "run 'httpserver -help' to see supported flags"
	case isAddrInUse(err):
		return "another process is using this port; try 'httpserver -port 8086' or '-port 0'"
	default:
		return ""
	}
}

func isAddrInUse(err error) bool {
	var opErr *net.OpError
	if !errors.As(err, &opErr) {
		return false
	}
	return errors.Is(opErr.Err, syscall.EADDRINUSE)
}
