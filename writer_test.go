package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSanitizeFileName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"myapp", "myapp"},
		{"my app", "myapp"},
		{"my/app", "myapp"},
		{"日本語サービス", ""},
		{"my-app_v2", "my-app_v2"},
		{"a.b.c", "abc"},
	}
	for _, tt := range tests {
		got := sanitizeFileName(tt.input)
		if got != tt.want {
			t.Errorf("sanitize(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestBuildFileName(t *testing.T) {
	tests := []struct {
		ns, svc, date string
		want          string
	}{
		{"prod", "myapp", "20260829", "prod-myapp-20260829.tsv"},
		{"", "myapp", "20260829", "myapp-20260829.tsv"},
		{"", "", "20260829", "unknown-20260829.tsv"},
		{"ns", "日本語サービス", "20260829", "ns-unknown-20260829.tsv"},
	}
	for _, tt := range tests {
		got := buildFileName(tt.ns, tt.svc, tt.date)
		if got != tt.want {
			t.Errorf("buildFileName(%q,%q,%q) = %q, want %q", tt.ns, tt.svc, tt.date, got, tt.want)
		}
	}
}

func TestWriterCreatesFileWithHeader(t *testing.T) {
	dir := t.TempDir()
	loc := time.UTC
	w := NewTSVWriter(dir, []string{"host.name"}, []string{"core"}, loc)
	defer w.Close()

	rows := []MetricRow{
		{
			Timestamp:          time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC),
			Name:               "cpu",
			Type:               "gauge",
			Value:              "50",
			Unit:               "%",
			ServiceNamespace:   "",
			ServiceName:        "web",
			ResourceAttributes: map[string]string{"host.name": "srv1"},
			Attributes:         map[string]string{"core": "0"},
		},
	}

	if err := w.WriteRows(rows); err != nil {
		t.Fatalf("WriteRows: %v", err)
	}
	w.Flush()

	path := filepath.Join(dir, "web-20260829.tsv")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	if !strings.HasPrefix(lines[0], "timestamp\t") {
		t.Errorf("header: %q", lines[0])
	}
	if !strings.Contains(lines[1], "cpu") {
		t.Errorf("data row: %q", lines[1])
	}
}

func TestWriterAppendsWithoutHeader(t *testing.T) {
	dir := t.TempDir()
	loc := time.UTC

	path := filepath.Join(dir, "web-20260829.tsv")
	os.WriteFile(path, []byte("timestamp\tname\ttype\tvalue\tunit\n"), 0644)

	w := NewTSVWriter(dir, nil, nil, loc)
	defer w.Close()

	rows := []MetricRow{
		{
			Timestamp:          time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC),
			Name:               "mem",
			Type:               "gauge",
			Value:              "1024",
			Unit:               "MB",
			ServiceName:        "web",
			ResourceAttributes: map[string]string{},
			Attributes:         map[string]string{},
		},
	}

	if err := w.WriteRows(rows); err != nil {
		t.Fatalf("WriteRows: %v", err)
	}
	w.Flush()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2 (original header + appended row)", len(lines))
	}
}

func TestWriterDifferentDates(t *testing.T) {
	dir := t.TempDir()
	loc := time.UTC

	w := NewTSVWriter(dir, nil, nil, loc)
	defer w.Close()

	rows := []MetricRow{
		{
			Timestamp:          time.Date(2026, 8, 29, 23, 59, 0, 0, time.UTC),
			Name:               "m1",
			Type:               "gauge",
			Value:              "1",
			ServiceName:        "svc",
			ResourceAttributes: map[string]string{},
			Attributes:         map[string]string{},
		},
		{
			Timestamp:          time.Date(2026, 8, 30, 0, 1, 0, 0, time.UTC),
			Name:               "m2",
			Type:               "gauge",
			Value:              "2",
			ServiceName:        "svc",
			ResourceAttributes: map[string]string{},
			Attributes:         map[string]string{},
		},
	}

	if err := w.WriteRows(rows); err != nil {
		t.Fatalf("WriteRows: %v", err)
	}
	w.Flush()

	if _, err := os.Stat(filepath.Join(dir, "svc-20260829.tsv")); err != nil {
		t.Errorf("expected svc-20260829.tsv: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "svc-20260830.tsv")); err != nil {
		t.Errorf("expected svc-20260830.tsv: %v", err)
	}
}

func TestWriterWithNamespace(t *testing.T) {
	dir := t.TempDir()
	loc := time.UTC

	w := NewTSVWriter(dir, nil, nil, loc)
	defer w.Close()

	rows := []MetricRow{
		{
			Timestamp:          time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC),
			Name:               "req",
			Type:               "counter",
			Value:              "100",
			ServiceNamespace:   "production",
			ServiceName:        "api",
			ResourceAttributes: map[string]string{},
			Attributes:         map[string]string{},
		},
	}

	if err := w.WriteRows(rows); err != nil {
		t.Fatalf("WriteRows: %v", err)
	}
	w.Flush()

	if _, err := os.Stat(filepath.Join(dir, "production-api-20260829.tsv")); err != nil {
		t.Errorf("expected production-api-20260829.tsv: %v", err)
	}
}

func TestWriterTimezoneAffectsFileName(t *testing.T) {
	dir := t.TempDir()
	jst := time.FixedZone("JST", 9*3600)

	w := NewTSVWriter(dir, nil, nil, jst)
	defer w.Close()

	// 2026-08-29 20:00 UTC = 2026-08-30 05:00 JST
	rows := []MetricRow{
		{
			Timestamp:          time.Date(2026, 8, 29, 20, 0, 0, 0, time.UTC),
			Name:               "m1",
			Type:               "gauge",
			Value:              "1",
			ServiceName:        "svc",
			ResourceAttributes: map[string]string{},
			Attributes:         map[string]string{},
		},
	}

	if err := w.WriteRows(rows); err != nil {
		t.Fatalf("WriteRows: %v", err)
	}
	w.Flush()

	// JST基準なので0830になるはず
	if _, err := os.Stat(filepath.Join(dir, "svc-20260830.tsv")); err != nil {
		t.Errorf("expected svc-20260830.tsv (JST): %v", err)
	}
}
