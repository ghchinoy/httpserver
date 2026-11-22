package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestRun_VersionFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "single_dash_version", args: []string{"-version"}},
		{name: "double_dash_version", args: []string{"--version"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if err := run(t.Context(), tc.args, &stdout, &stderr); err != nil {
				t.Fatalf("run(%v) returned unexpected error: %v", tc.args, err)
			}
			if got := stdout.String(); !strings.HasPrefix(got, "httpserver ") {
				t.Errorf("run(%v) stdout = %q, want prefix %q", tc.args, got, "httpserver ")
			}
		})
	}
}

func TestRun_HelpFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run(t.Context(), []string{"-help"}, &stdout, &stderr); err != nil {
		t.Fatalf("run(%v) returned unexpected error: %v", []string{"-help"}, err)
	}
	if got := stderr.String(); !strings.Contains(got, "Usage: httpserver [flags]") {
		t.Errorf("run(%v) stderr = %q, want substring %q", []string{"-help"}, got, "Usage: httpserver [flags]")
	}
}

func TestRun_InvalidArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr error
	}{
		{
			name:    "unexpected_positional_arg",
			args:    []string{"extra"},
			wantErr: errUnexpectedArgs,
		},
		{
			name:    "invalid_port",
			args:    []string{"-port", "99999"},
			wantErr: errInvalidPort,
		},
		{
			name:    "conflicting_cors_flags",
			args:    []string{"-cors", "-cors-origin", "http://localhost:3000"},
			wantErr: errConflictingCORSFlags,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := run(t.Context(), tc.args, &stdout, &stderr)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("run(%v) = %v, want %v", tc.args, err, tc.wantErr)
			}
			if hint := errorHint(err); hint == "" {
				t.Errorf("errorHint(%v) = %q, want non-empty hint", err, hint)
			}
		})
	}
}
