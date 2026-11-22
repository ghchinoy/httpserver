package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	defaultHost       = "127.0.0.1"
	defaultPort       = 8085
	defaultWebDir     = "."
	logFormatText     = "text"
	logFormatJSON     = "json"
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 60 * time.Second
	idleTimeout       = 120 * time.Second
	shutdownTimeout   = 5 * time.Second
	maxHeaderBytes    = 1 << 20 // 1 MiB
)

var (
	errInvalidPort          = errors.New("port must be between 0 and 65535")
	errWebDirNotFound       = errors.New("web directory does not exist")
	errWebDirNotDirectory   = errors.New("web path is not a directory")
	errConflictingCORSFlags = errors.New("cannot specify both -cors and -cors-origin")
	errInvalidCORSOrigin    = errors.New("invalid CORS origin")
	errInvalidLogFormat     = errors.New("unsupported log format")
	errUnexpectedArgs       = errors.New("unexpected positional arguments")
)

type config struct {
	host          string
	port          int
	webDir        string
	corsWildcard  bool
	corsOrigin    string
	allowDotfiles bool
	noDirListing  bool
	logFormat     string
	quiet         bool
}

func (c config) effectiveCORSOrigin() string {
	if c.corsWildcard {
		return "*"
	}
	return c.corsOrigin
}

func validateConfig(cfg config) error {
	if cfg.port < 0 || cfg.port > 65535 {
		return fmt.Errorf("%w: got %d", errInvalidPort, cfg.port)
	}

	if cfg.corsWildcard && cfg.corsOrigin != "" {
		return errConflictingCORSFlags
	}

	if cfg.corsOrigin != "" && hasControlChars(cfg.corsOrigin) {
		return fmt.Errorf("%w: contains control characters", errInvalidCORSOrigin)
	}

	if cfg.logFormat != logFormatText && cfg.logFormat != logFormatJSON {
		return fmt.Errorf("%w: %q", errInvalidLogFormat, cfg.logFormat)
	}

	info, err := os.Stat(cfg.webDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: %q", errWebDirNotFound, cfg.webDir)
		}
		return fmt.Errorf("inspecting web directory %q: %w", cfg.webDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: %q", errWebDirNotDirectory, cfg.webDir)
	}

	return nil
}

func hasControlChars(s string) bool {
	for _, r := range s {
		if r <= 0x1F || r == 0x7F {
			return true
		}
	}
	return false
}

func isLoopbackHost(host string) bool {
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func serve(ctx context.Context, cfg config, logger *slog.Logger) error {
	absWebDir, err := filepath.Abs(cfg.webDir)
	if err != nil {
		return fmt.Errorf("resolving web directory %q: %w", cfg.webDir, err)
	}

	root, err := os.OpenRoot(absWebDir)
	if err != nil {
		return fmt.Errorf("opening web root %q: %w", absWebDir, err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			logger.Warn("closing web root failed", "error", closeErr)
		}
	}()

	fsys := &filteredFS{
		base:          root.FS(),
		allowDotfiles: cfg.allowDotfiles,
		noDirListing:  cfg.noDirListing,
	}

	addr := net.JoinHostPort(cfg.host, strconv.Itoa(cfg.port))
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("listening on %q: %w", addr, err)
	}

	handler := newHandler(fsys, cfg, logger)
	srv := newHTTPServer(ln.Addr().String(), handler, logger)

	logStartup(logger, ln.Addr().String(), absWebDir, cfg)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ln)
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serving HTTP: %w", err)
	case <-ctx.Done():
		logger.Info("shutting down server")
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("graceful shutdown: %w", err)
		}
		if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serving HTTP: %w", err)
		}
		return nil
	}
}

func logStartup(logger *slog.Logger, listenAddr, webDir string, cfg config) {
	corsOrigin := cfg.effectiveCORSOrigin()
	displayHost, displayPort, err := net.SplitHostPort(listenAddr)
	urlStr := "http://" + listenAddr
	if err == nil {
		if displayHost == "" || displayHost == "0.0.0.0" || displayHost == "::" {
			displayHost = "localhost"
		}
		urlStr = "http://" + net.JoinHostPort(displayHost, displayPort)
	}

	if !isLoopbackHost(cfg.host) {
		logger.Warn("server is bound to a non-loopback address and reachable over the network",
			"addr", listenAddr,
		)
	}
	if corsOrigin == "*" {
		logger.Warn("permissive wildcard CORS is enabled; any webpage can read served files")
	}
	if cfg.allowDotfiles {
		logger.Warn("dotfile serving is enabled; hidden files such as .env or .git may be exposed")
	}

	logger.Info("server listening",
		"url", urlStr,
		"addr", listenAddr,
		"dir", webDir,
		"cors", corsOrigin,
		"dotfiles", cfg.allowDotfiles,
		"dir_listing", !cfg.noDirListing,
	)
}

func newHTTPServer(addr string, handler http.Handler, logger *slog.Logger) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}
}

func newHandler(fsys fs.FS, cfg config, logger *slog.Logger) http.Handler {
	h := http.FileServerFS(fsys)
	h = methodsMiddleware(h)
	if origin := cfg.effectiveCORSOrigin(); origin != "" {
		h = corsMiddleware(origin, h)
	}
	h = securityHeadersMiddleware(h)
	if !cfg.quiet {
		h = requestLoggerMiddleware(logger, h)
	}
	return h
}

// filteredFS wraps an fs.FS to optionally hide dotfiles and suppress directory listings.
type filteredFS struct {
	base          fs.FS
	allowDotfiles bool
	noDirListing  bool
}

// Open opens the named file after applying dotfile and directory-listing policies.
func (f *filteredFS) Open(name string) (fs.File, error) {
	if !f.allowDotfiles && containsDotSegment(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	file, err := f.base.Open(name)
	if err != nil {
		// Normalize os.Root symlink-escape errors (and other non-permission open
		// failures) to fs.ErrNotExist so http.FileServerFS responds with 404
		// instead of 500 and does not leak symlink topology.
		if errors.Is(err, fs.ErrPermission) {
			return nil, err
		}
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	if !f.noDirListing {
		return file, nil
	}

	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if !info.IsDir() {
		return file, nil
	}

	// Allow directories that contain an index.html so http.FileServerFS can serve it.
	indexPath := "index.html"
	if name != "." && name != "" {
		indexPath = path.Join(name, "index.html")
	}
	indexFile, indexErr := f.base.Open(indexPath)
	if indexErr != nil {
		_ = file.Close()
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	_ = indexFile.Close()
	return file, nil
}

func containsDotSegment(name string) bool {
	if name == "." || name == "" {
		return false
	}
	for seg := range strings.SplitSeq(name, "/") {
		if strings.HasPrefix(seg, ".") {
			return true
		}
	}
	return false
}
