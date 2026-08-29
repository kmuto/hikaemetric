package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cpb "go.opentelemetry.io/proto/otlp/common/v1"
	collpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	mpb "go.opentelemetry.io/proto/otlp/metrics/v1"
	rpb "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/protobuf/proto"
)

func TestHandleMetrics(t *testing.T) {
	dir := t.TempDir()
	loc := time.UTC
	w := NewTSVWriter(dir, []string{"host.name"}, []string{"method"}, loc)
	defer w.Close()

	srv := NewServer(":0", w, loc)

	ts := uint64(time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC).UnixNano())
	req := &collpb.ExportMetricsServiceRequest{
		ResourceMetrics: []*mpb.ResourceMetrics{
			{
				Resource: &rpb.Resource{
					Attributes: []*cpb.KeyValue{
						{Key: "service.name", Value: &cpb.AnyValue{Value: &cpb.AnyValue_StringValue{StringValue: "testapp"}}},
						{Key: "host.name", Value: &cpb.AnyValue{Value: &cpb.AnyValue_StringValue{StringValue: "host1"}}},
					},
				},
				ScopeMetrics: []*mpb.ScopeMetrics{
					{
						Metrics: []*mpb.Metric{
							{
								Name: "http.requests",
								Unit: "1",
								Data: &mpb.Metric_Sum{
									Sum: &mpb.Sum{
										DataPoints: []*mpb.NumberDataPoint{
											{
												TimeUnixNano: ts,
												Value:        &mpb.NumberDataPoint_AsInt{AsInt: 42},
												Attributes: []*cpb.KeyValue{
													{Key: "method", Value: &cpb.AnyValue{Value: &cpb.AnyValue_StringValue{StringValue: "GET"}}},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	body, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	httpReq := httptest.NewRequest(http.MethodPost, "/v1/metrics", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/x-protobuf")
	rec := httptest.NewRecorder()

	srv.handleMetrics(rec, httpReq)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d, body: %s", rec.Code, rec.Body.String())
	}

	resp := &collpb.ExportMetricsServiceResponse{}
	if err := proto.Unmarshal(rec.Body.Bytes(), resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	w.Flush()

	path := filepath.Join(dir, "testapp-20260829.tsv")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read tsv: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}

	wantHeader := "timestamp\tname\ttype\tvalue\tunit\tresource.host.name\tmethod"
	if lines[0] != wantHeader {
		t.Errorf("header:\ngot:  %q\nwant: %q", lines[0], wantHeader)
	}

	if !strings.Contains(lines[1], "http.requests") || !strings.Contains(lines[1], "42") || !strings.Contains(lines[1], "host1") || !strings.Contains(lines[1], "GET") {
		t.Errorf("data row: %q", lines[1])
	}
}

func TestHandleMetricsMethodNotAllowed(t *testing.T) {
	dir := t.TempDir()
	w := NewTSVWriter(dir, nil, nil, time.UTC)
	defer w.Close()

	srv := NewServer(":0", w, time.UTC)
	httpReq := httptest.NewRequest(http.MethodGet, "/v1/metrics", nil)
	rec := httptest.NewRecorder()
	srv.handleMetrics(rec, httpReq)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleMetricsInvalidBody(t *testing.T) {
	dir := t.TempDir()
	w := NewTSVWriter(dir, nil, nil, time.UTC)
	defer w.Close()

	srv := NewServer(":0", w, time.UTC)
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/metrics", bytes.NewReader([]byte("not protobuf")))
	rec := httptest.NewRecorder()
	srv.handleMetrics(rec, httpReq)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
