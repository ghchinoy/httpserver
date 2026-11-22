package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testLogger(buf *bytes.Buffer, format string) *slog.Logger {
	return newLogger(buf, format, false)
}

func setupTestWebDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	files := map[string]string{
		"index.html":           "<h1>Home</h1>",
		"hello.txt":            "hello world",
		".env":                 "SECRET=do-not-leak",
		".git/config":          "[core]\nrepositoryformatversion = 0\n",
		"sub/page.html":        "<p>Sub Page</p>",
		"sub/.hidden":          "hidden value",
		"withindex/index.html": "<h1>Nested Index</h1>",
	}

	for relPath, content := range files {
		fullPath := filepath.Join(dir, filepath.FromSlash(relPath))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
			t.Fatalf("os.MkdirAll(%q) returned unexpected error: %v", filepath.Dir(fullPath), err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o600); err != nil {
			t.Fatalf("os.WriteFile(%q) returned unexpected error: %v", fullPath, err)
		}
	}

	return dir
}

func TestValidateConfig(t *testing.T) {
	validDir := t.TempDir()
	regularFile := filepath.Join(validDir, "file.txt")
	if err := os.WriteFile(regularFile, []byte("x"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) returned unexpected error: %v", regularFile, err)
	}

	tests := []struct {
		name    string
		cfg     config
		wantErr error
	}{
		{
			name: "valid_defaults",
			cfg: config{
				host:      defaultHost,
				port:      defaultPort,
				webDir:    validDir,
				logFormat: logFormatText,
			},
		},
		{
			name: "valid_ephemeral_port_and_json",
			cfg: config{
				host:       defaultHost,
				port:       0,
				webDir:     validDir,
				corsOrigin: "http://localhost:3000",
				logFormat:  logFormatJSON,
			},
		},
		{
			name: "negative_port",
			cfg: config{
				host:      defaultHost,
				port:      -1,
				webDir:    validDir,
				logFormat: logFormatText,
			},
			wantErr: errInvalidPort,
		},
		{
			name: "port_above_max",
			cfg: config{
				host:      defaultHost,
				port:      65536,
				webDir:    validDir,
				logFormat: logFormatText,
			},
			wantErr: errInvalidPort,
		},
		{
			name: "conflicting_cors_flags",
			cfg: config{
				host:         defaultHost,
				port:         8085,
				webDir:       validDir,
				corsWildcard: true,
				corsOrigin:   "http://localhost:3000",
				logFormat:    logFormatText,
			},
			wantErr: errConflictingCORSFlags,
		},
		{
			name: "cors_origin_crlf_injection",
			cfg: config{
				host:       defaultHost,
				port:       8085,
				webDir:     validDir,
				corsOrigin: "http://localhost:3000\r\nX-Injected: yes",
				logFormat:  logFormatText,
			},
			wantErr: errInvalidCORSOrigin,
		},
		{
			name: "unsupported_log_format",
			cfg: config{
				host:      defaultHost,
				port:      8085,
				webDir:    validDir,
				logFormat: "yaml",
			},
			wantErr: errInvalidLogFormat,
		},
		{
			name: "missing_web_dir",
			cfg: config{
				host:      defaultHost,
				port:      8085,
				webDir:    filepath.Join(validDir, "does-not-exist"),
				logFormat: logFormatText,
			},
			wantErr: errWebDirNotFound,
		},
		{
			name: "web_path_is_regular_file",
			cfg: config{
				host:      defaultHost,
				port:      8085,
				webDir:    regularFile,
				logFormat: logFormatText,
			},
			wantErr: errWebDirNotDirectory,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateConfig(tc.cfg)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("validateConfig(%+v) = %v, want %v", tc.cfg, err, tc.wantErr)
			}
		})
	}
}

func TestIsLoopbackHost(t *testing.T) {
	tests := []struct {
		name string
		host string
		want bool
	}{
		{name: "ipv4_loopback", host: "127.0.0.1", want: true},
		{name: "ipv6_loopback", host: "::1", want: true},
		{name: "localhost_lower", host: "localhost", want: true},
		{name: "localhost_upper", host: "LOCALHOST", want: true},
		{name: "empty_all_interfaces", host: "", want: false},
		{name: "ipv4_wildcard", host: "0.0.0.0", want: false},
		{name: "ipv6_wildcard", host: "::", want: false},
		{name: "lan_address", host: "192.168.1.10", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isLoopbackHost(tc.host); got != tc.want {
				t.Errorf("isLoopbackHost(%q) = %v, want %v", tc.host, got, tc.want)
			}
		})
	}
}

func TestHandler_SecurityAndFiles(t *testing.T) {
	webDir := setupTestWebDir(t)

	// Create a secret file outside webDir and a symlink inside webDir pointing to it.
	outsideDir := t.TempDir()
	outsideSecret := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(outsideSecret, []byte("top secret"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) returned unexpected error: %v", outsideSecret, err)
	}
	escapeLink := filepath.Join(webDir, "escape.txt")
	if err := os.Symlink(outsideSecret, escapeLink); err != nil {
		t.Fatalf("os.Symlink(%q, %q) returned unexpected error: %v", outsideSecret, escapeLink, err)
	}

	root, err := os.OpenRoot(webDir)
	if err != nil {
		t.Fatalf("os.OpenRoot(%q) returned unexpected error: %v", webDir, err)
	}
	t.Cleanup(func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Errorf("root.Close() returned unexpected error: %v", closeErr)
		}
	})

	tests := []struct {
		name           string
		cfg            config
		method         string
		target         string
		wantStatus     int
		wantBodySubstr string
		wantCORSOrigin string
		wantVary       string
		wantAllow      string
	}{
		{
			name:           "serve_static_file",
			cfg:            config{logFormat: logFormatText},
			method:         http.MethodGet,
			target:         "/hello.txt",
			wantStatus:     http.StatusOK,
			wantBodySubstr: "hello world",
		},
		{
			name:       "head_request_allowed",
			cfg:        config{logFormat: logFormatText},
			method:     http.MethodHead,
			target:     "/hello.txt",
			wantStatus: http.StatusOK,
		},
		{
			name:       "post_rejected_method_not_allowed",
			cfg:        config{logFormat: logFormatText},
			method:     http.MethodPost,
			target:     "/hello.txt",
			wantStatus: http.StatusMethodNotAllowed,
			wantAllow:  allowedMethodsHeader,
		},
		{
			name:       "delete_rejected_method_not_allowed",
			cfg:        config{logFormat: logFormatText},
			method:     http.MethodDelete,
			target:     "/hello.txt",
			wantStatus: http.StatusMethodNotAllowed,
			wantAllow:  allowedMethodsHeader,
		},
		{
			name:       "dotfile_env_blocked_by_default",
			cfg:        config{logFormat: logFormatText},
			method:     http.MethodGet,
			target:     "/.env",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "dotdir_git_config_blocked_by_default",
			cfg:        config{logFormat: logFormatText},
			method:     http.MethodGet,
			target:     "/.git/config",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "nested_dotfile_blocked_by_default",
			cfg:        config{logFormat: logFormatText},
			method:     http.MethodGet,
			target:     "/sub/.hidden",
			wantStatus: http.StatusNotFound,
		},
		{
			name:           "dotfile_allowed_when_opted_in",
			cfg:            config{allowDotfiles: true, logFormat: logFormatText},
			method:         http.MethodGet,
			target:         "/.env",
			wantStatus:     http.StatusOK,
			wantBodySubstr: "SECRET=do-not-leak",
		},
		{
			name:       "symlink_escape_blocked_by_os_root",
			cfg:        config{logFormat: logFormatText},
			method:     http.MethodGet,
			target:     "/escape.txt",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "path_traversal_blocked",
			cfg:        config{logFormat: logFormatText},
			method:     http.MethodGet,
			target:     "/../secret.txt",
			wantStatus: http.StatusNotFound,
		},
		{
			name:           "dir_listing_enabled_by_default",
			cfg:            config{logFormat: logFormatText},
			method:         http.MethodGet,
			target:         "/sub/",
			wantStatus:     http.StatusOK,
			wantBodySubstr: "page.html",
		},
		{
			name:       "dir_listing_disabled_blocks_directory_without_index",
			cfg:        config{noDirListing: true, logFormat: logFormatText},
			method:     http.MethodGet,
			target:     "/sub/",
			wantStatus: http.StatusNotFound,
		},
		{
			name:           "dir_listing_disabled_allows_directory_with_index",
			cfg:            config{noDirListing: true, logFormat: logFormatText},
			method:         http.MethodGet,
			target:         "/withindex/",
			wantStatus:     http.StatusOK,
			wantBodySubstr: "Nested Index",
		},
		{
			name:           "dir_listing_disabled_still_serves_nested_file",
			cfg:            config{noDirListing: true, logFormat: logFormatText},
			method:         http.MethodGet,
			target:         "/sub/page.html",
			wantStatus:     http.StatusOK,
			wantBodySubstr: "Sub Page",
		},
		{
			name:           "cors_disabled_by_default",
			cfg:            config{logFormat: logFormatText},
			method:         http.MethodGet,
			target:         "/hello.txt",
			wantStatus:     http.StatusOK,
			wantCORSOrigin: "",
		},
		{
			name:           "cors_wildcard_preflight",
			cfg:            config{corsWildcard: true, logFormat: logFormatText},
			method:         http.MethodOptions,
			target:         "/hello.txt",
			wantStatus:     http.StatusNoContent,
			wantCORSOrigin: "*",
		},
		{
			name:           "cors_specific_origin_sets_vary",
			cfg:            config{corsOrigin: "http://localhost:3000", logFormat: logFormatText},
			method:         http.MethodGet,
			target:         "/hello.txt",
			wantStatus:     http.StatusOK,
			wantCORSOrigin: "http://localhost:3000",
			wantVary:       "Origin",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var logBuf bytes.Buffer
			fsys := &filteredFS{
				base:          root.FS(),
				allowDotfiles: tc.cfg.allowDotfiles,
				noDirListing:  tc.cfg.noDirListing,
			}
			handler := newHandler(fsys, tc.cfg, testLogger(&logBuf, tc.cfg.logFormat))

			req := httptest.NewRequest(tc.method, tc.target, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			res := rec.Result()
			defer func() {
				if closeErr := res.Body.Close(); closeErr != nil {
					t.Errorf("res.Body.Close() returned unexpected error: %v", closeErr)
				}
			}()

			if res.StatusCode != tc.wantStatus {
				t.Errorf("ServeHTTP(%s %q) status = %d, want %d", tc.method, tc.target, res.StatusCode, tc.wantStatus)
			}

			if got := res.Header.Get("X-Content-Type-Options"); got != "nosniff" {
				t.Errorf("ServeHTTP(%s %q) X-Content-Type-Options = %q, want %q", tc.method, tc.target, got, "nosniff")
			}
			if got := res.Header.Get("X-Frame-Options"); got != "SAMEORIGIN" {
				t.Errorf("ServeHTTP(%s %q) X-Frame-Options = %q, want %q", tc.method, tc.target, got, "SAMEORIGIN")
			}
			if got := res.Header.Get("Referrer-Policy"); got != "no-referrer" {
				t.Errorf("ServeHTTP(%s %q) Referrer-Policy = %q, want %q", tc.method, tc.target, got, "no-referrer")
			}

			if got := res.Header.Get("Access-Control-Allow-Origin"); got != tc.wantCORSOrigin {
				t.Errorf("ServeHTTP(%s %q) Access-Control-Allow-Origin = %q, want %q", tc.method, tc.target, got, tc.wantCORSOrigin)
			}
			if tc.wantVary != "" {
				if got := res.Header.Get("Vary"); got != tc.wantVary {
					t.Errorf("ServeHTTP(%s %q) Vary = %q, want %q", tc.method, tc.target, got, tc.wantVary)
				}
			}
			if tc.wantAllow != "" {
				if got := res.Header.Get("Allow"); got != tc.wantAllow {
					t.Errorf("ServeHTTP(%s %q) Allow = %q, want %q", tc.method, tc.target, got, tc.wantAllow)
				}
			}

			if tc.wantBodySubstr != "" {
				body, err := io.ReadAll(res.Body)
				if err != nil {
					t.Fatalf("io.ReadAll(res.Body) returned unexpected error: %v", err)
				}
				if !strings.Contains(string(body), tc.wantBodySubstr) {
					t.Errorf("ServeHTTP(%s %q) body = %q, want substring %q", tc.method, tc.target, string(body), tc.wantBodySubstr)
				}
			}
		})
	}
}

func TestRequestLoggerMiddleware_JSONFormat(t *testing.T) {
	var logBuf bytes.Buffer
	logger := testLogger(&logBuf, logFormatJSON)

	handler := requestLoggerMiddleware(logger, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.WriteHeader(http.StatusInternalServerError) // duplicate WriteHeader must be ignored
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Errorf("w.Write(%q) returned unexpected error: %v", "ok", err)
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var entry struct {
		Msg    string `json:"msg"`
		Method string `json:"method"`
		Path   string `json:"path"`
		Status int    `json:"status"`
		Bytes  int64  `json:"bytes"`
	}
	if err := json.Unmarshal(logBuf.Bytes(), &entry); err != nil {
		t.Fatalf("json.Unmarshal(%q) returned unexpected error: %v", logBuf.String(), err)
	}

	if entry.Method != http.MethodGet || entry.Path != "/test" || entry.Status != http.StatusCreated || entry.Bytes != 2 {
		t.Errorf("requestLoggerMiddleware logged %+v, want method=GET path=/test status=201 bytes=2", entry)
	}
}

func TestStatusResponseWriter_Unwrap(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &statusResponseWriter{ResponseWriter: rec, status: http.StatusOK}
	if got := rw.Unwrap(); got != rec {
		t.Errorf("rw.Unwrap() = %v, want %v", got, rec)
	}
}

func TestServe_GracefulShutdown(t *testing.T) {
	webDir := setupTestWebDir(t)
	ctx, cancel := context.WithCancel(t.Context())

	var logBuf bytes.Buffer
	cfg := config{
		host:      "127.0.0.1",
		port:      0,
		webDir:    webDir,
		logFormat: logFormatJSON,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- serve(ctx, cfg, testLogger(&logBuf, logFormatJSON))
	}()

	// Cancel immediately once the goroutine has had a chance to start; serve handles
	// both pre- and post-listen cancellation cleanly.
	cancel()
	if err := <-errCh; err != nil {
		t.Errorf("serve(%+v) returned unexpected error on graceful shutdown: %v", cfg, err)
	}
}

func TestNewHTTPServer_TimeoutsConfigured(t *testing.T) {
	var logBuf bytes.Buffer
	const addr = "127.0.0.1:0"
	srv := newHTTPServer(addr, http.NotFoundHandler(), testLogger(&logBuf, logFormatText))

	if srv.ReadHeaderTimeout != readHeaderTimeout {
		t.Errorf("newHTTPServer(%q).ReadHeaderTimeout = %v, want %v", addr, srv.ReadHeaderTimeout, readHeaderTimeout)
	}
	if srv.ReadTimeout != readTimeout {
		t.Errorf("newHTTPServer(%q).ReadTimeout = %v, want %v", addr, srv.ReadTimeout, readTimeout)
	}
	if srv.WriteTimeout != writeTimeout {
		t.Errorf("newHTTPServer(%q).WriteTimeout = %v, want %v", addr, srv.WriteTimeout, writeTimeout)
	}
	if srv.IdleTimeout != idleTimeout {
		t.Errorf("newHTTPServer(%q).IdleTimeout = %v, want %v", addr, srv.IdleTimeout, idleTimeout)
	}
	if srv.MaxHeaderBytes != maxHeaderBytes {
		t.Errorf("newHTTPServer(%q).MaxHeaderBytes = %d, want %d", addr, srv.MaxHeaderBytes, maxHeaderBytes)
	}
}
