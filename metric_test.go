package main

import (
	"encoding/json"
	"testing"
	"time"

	cpb "go.opentelemetry.io/proto/otlp/common/v1"
	mpb "go.opentelemetry.io/proto/otlp/metrics/v1"
	rpb "go.opentelemetry.io/proto/otlp/resource/v1"
)

func TestAnyValueToString(t *testing.T) {
	tests := []struct {
		name string
		val  *cpb.AnyValue
		want string
	}{
		{"nil", nil, ""},
		{"string", &cpb.AnyValue{Value: &cpb.AnyValue_StringValue{StringValue: "hello"}}, "hello"},
		{"int", &cpb.AnyValue{Value: &cpb.AnyValue_IntValue{IntValue: 42}}, "42"},
		{"double", &cpb.AnyValue{Value: &cpb.AnyValue_DoubleValue{DoubleValue: 3.14}}, "3.14"},
		{"bool", &cpb.AnyValue{Value: &cpb.AnyValue_BoolValue{BoolValue: true}}, "true"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := anyValueToString(tt.val)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractServiceInfo(t *testing.T) {
	res := &rpb.Resource{
		Attributes: []*cpb.KeyValue{
			{Key: "service.namespace", Value: &cpb.AnyValue{Value: &cpb.AnyValue_StringValue{StringValue: "prod"}}},
			{Key: "service.name", Value: &cpb.AnyValue{Value: &cpb.AnyValue_StringValue{StringValue: "myapp"}}},
			{Key: "host.name", Value: &cpb.AnyValue{Value: &cpb.AnyValue_StringValue{StringValue: "server01"}}},
		},
	}

	ns, name, attrs := extractServiceInfo(res)
	if ns != "prod" {
		t.Errorf("namespace: got %q, want %q", ns, "prod")
	}
	if name != "myapp" {
		t.Errorf("name: got %q, want %q", name, "myapp")
	}
	if attrs["host.name"] != "server01" {
		t.Errorf("host.name: got %q, want %q", attrs["host.name"], "server01")
	}
}

func TestExtractServiceInfoNilResource(t *testing.T) {
	ns, name, _ := extractServiceInfo(nil)
	if ns != "" {
		t.Errorf("namespace: got %q, want empty", ns)
	}
	if name != "unknown" {
		t.Errorf("name: got %q, want %q", name, "unknown")
	}
}

func TestConvertMetricsGauge(t *testing.T) {
	loc := time.UTC
	ts := uint64(time.Date(2026, 8, 29, 10, 30, 0, 0, time.UTC).UnixNano())

	rm := []*mpb.ResourceMetrics{
		{
			Resource: &rpb.Resource{
				Attributes: []*cpb.KeyValue{
					{Key: "service.name", Value: &cpb.AnyValue{Value: &cpb.AnyValue_StringValue{StringValue: "web"}}},
				},
			},
			ScopeMetrics: []*mpb.ScopeMetrics{
				{
					Metrics: []*mpb.Metric{
						{
							Name: "cpu.usage",
							Unit: "%",
							Data: &mpb.Metric_Gauge{
								Gauge: &mpb.Gauge{
									DataPoints: []*mpb.NumberDataPoint{
										{
											TimeUnixNano: ts,
											Value:        &mpb.NumberDataPoint_AsDouble{AsDouble: 75.5},
											Attributes: []*cpb.KeyValue{
												{Key: "core", Value: &cpb.AnyValue{Value: &cpb.AnyValue_StringValue{StringValue: "0"}}},
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

	rows := convertMetrics(rm, loc)
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}

	r := rows[0]
	if r.Name != "cpu.usage" {
		t.Errorf("name: got %q", r.Name)
	}
	if r.Type != "gauge" {
		t.Errorf("type: got %q", r.Type)
	}
	if r.Value != "75.5" {
		t.Errorf("value: got %q", r.Value)
	}
	if r.Unit != "%" {
		t.Errorf("unit: got %q", r.Unit)
	}
	if r.ServiceName != "web" {
		t.Errorf("svc: got %q", r.ServiceName)
	}
	if r.Attributes["core"] != "0" {
		t.Errorf("attr core: got %q", r.Attributes["core"])
	}
}

func TestConvertMetricsCounter(t *testing.T) {
	loc := time.UTC
	ts := uint64(time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC).UnixNano())

	rm := []*mpb.ResourceMetrics{
		{
			Resource: &rpb.Resource{
				Attributes: []*cpb.KeyValue{
					{Key: "service.name", Value: &cpb.AnyValue{Value: &cpb.AnyValue_StringValue{StringValue: "api"}}},
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
											Value:        &mpb.NumberDataPoint_AsInt{AsInt: 1234},
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

	rows := convertMetrics(rm, loc)
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0].Type != "counter" {
		t.Errorf("type: got %q", rows[0].Type)
	}
	if rows[0].Value != "1234" {
		t.Errorf("value: got %q", rows[0].Value)
	}
}

func TestConvertMetricsHistogram(t *testing.T) {
	loc := time.UTC
	ts := uint64(time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC).UnixNano())
	sum := 150.0

	rm := []*mpb.ResourceMetrics{
		{
			Resource: &rpb.Resource{
				Attributes: []*cpb.KeyValue{
					{Key: "service.name", Value: &cpb.AnyValue{Value: &cpb.AnyValue_StringValue{StringValue: "api"}}},
				},
			},
			ScopeMetrics: []*mpb.ScopeMetrics{
				{
					Metrics: []*mpb.Metric{
						{
							Name: "http.duration",
							Unit: "ms",
							Data: &mpb.Metric_Histogram{
								Histogram: &mpb.Histogram{
									DataPoints: []*mpb.HistogramDataPoint{
										{
											TimeUnixNano:   ts,
											Count:          100,
											Sum:            &sum,
											BucketCounts:   []uint64{10, 30, 40, 20},
											ExplicitBounds: []float64{10, 50, 100},
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

	rows := convertMetrics(rm, loc)
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0].Type != "histogram" {
		t.Errorf("type: got %q", rows[0].Type)
	}

	var hv HistogramValue
	if err := json.Unmarshal([]byte(rows[0].Value), &hv); err != nil {
		t.Fatalf("unmarshal histogram: %v", err)
	}
	if hv.Count != 100 {
		t.Errorf("count: got %d", hv.Count)
	}
	if *hv.Sum != 150.0 {
		t.Errorf("sum: got %f", *hv.Sum)
	}
}

func TestFormatRow(t *testing.T) {
	row := MetricRow{
		Timestamp:          time.Date(2026, 8, 29, 10, 30, 0, 0, time.UTC),
		Name:               "cpu.usage",
		Type:               "gauge",
		Value:              "75.5",
		Unit:               "%",
		ResourceAttributes: map[string]string{"host.name": "server01"},
		Attributes:         map[string]string{"core": "0"},
	}

	line := formatRow(row, []string{"host.name"}, []string{"core"})
	want := "2026-08-29T10:30:00Z\tcpu.usage\tgauge\t75.5\t%\tserver01\t0"
	if line != want {
		t.Errorf("got:\n%s\nwant:\n%s", line, want)
	}
}

func TestHeaderLine(t *testing.T) {
	h := headerLine([]string{"host.name"}, []string{"core", "region"})
	want := "timestamp\tname\ttype\tvalue\tunit\tresource.host.name\tcore\tregion"
	if h != want {
		t.Errorf("got:\n%s\nwant:\n%s", h, want)
	}
}

func TestTSVOutputByMetricType(t *testing.T) {
	loc := time.UTC
	ts := uint64(time.Date(2026, 8, 29, 15, 0, 0, 0, time.UTC).UnixNano())
	resourceAttrKeys := []string{"host.name"}
	attrKeys := []string{"method"}

	resource := &rpb.Resource{
		Attributes: []*cpb.KeyValue{
			{Key: "service.name", Value: &cpb.AnyValue{Value: &cpb.AnyValue_StringValue{StringValue: "myapp"}}},
			{Key: "host.name", Value: &cpb.AnyValue{Value: &cpb.AnyValue_StringValue{StringValue: "host1"}}},
		},
	}
	methodAttr := []*cpb.KeyValue{
		{Key: "method", Value: &cpb.AnyValue{Value: &cpb.AnyValue_StringValue{StringValue: "GET"}}},
	}

	t.Run("gauge_double", func(t *testing.T) {
		rm := []*mpb.ResourceMetrics{{
			Resource: resource,
			ScopeMetrics: []*mpb.ScopeMetrics{{Metrics: []*mpb.Metric{{
				Name: "cpu.usage",
				Unit: "%",
				Data: &mpb.Metric_Gauge{Gauge: &mpb.Gauge{DataPoints: []*mpb.NumberDataPoint{{
					TimeUnixNano: ts,
					Value:        &mpb.NumberDataPoint_AsDouble{AsDouble: 82.3},
					Attributes:   methodAttr,
				}}}},
			}}}},
		}}
		rows := convertMetrics(rm, loc)
		got := formatRow(rows[0], resourceAttrKeys, attrKeys)
		want := "2026-08-29T15:00:00Z\tcpu.usage\tgauge\t82.3\t%\thost1\tGET"
		if got != want {
			t.Errorf("got:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("gauge_int", func(t *testing.T) {
		rm := []*mpb.ResourceMetrics{{
			Resource: resource,
			ScopeMetrics: []*mpb.ScopeMetrics{{Metrics: []*mpb.Metric{{
				Name: "goroutines",
				Unit: "1",
				Data: &mpb.Metric_Gauge{Gauge: &mpb.Gauge{DataPoints: []*mpb.NumberDataPoint{{
					TimeUnixNano: ts,
					Value:        &mpb.NumberDataPoint_AsInt{AsInt: 42},
					Attributes:   methodAttr,
				}}}},
			}}}},
		}}
		rows := convertMetrics(rm, loc)
		got := formatRow(rows[0], resourceAttrKeys, attrKeys)
		want := "2026-08-29T15:00:00Z\tgoroutines\tgauge\t42\t1\thost1\tGET"
		if got != want {
			t.Errorf("got:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("counter", func(t *testing.T) {
		rm := []*mpb.ResourceMetrics{{
			Resource: resource,
			ScopeMetrics: []*mpb.ScopeMetrics{{Metrics: []*mpb.Metric{{
				Name: "http.requests.total",
				Unit: "1",
				Data: &mpb.Metric_Sum{Sum: &mpb.Sum{DataPoints: []*mpb.NumberDataPoint{{
					TimeUnixNano: ts,
					Value:        &mpb.NumberDataPoint_AsInt{AsInt: 9876},
					Attributes:   methodAttr,
				}}}},
			}}}},
		}}
		rows := convertMetrics(rm, loc)
		got := formatRow(rows[0], resourceAttrKeys, attrKeys)
		want := "2026-08-29T15:00:00Z\thttp.requests.total\tcounter\t9876\t1\thost1\tGET"
		if got != want {
			t.Errorf("got:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("histogram", func(t *testing.T) {
		sum := 350.5
		rm := []*mpb.ResourceMetrics{{
			Resource: resource,
			ScopeMetrics: []*mpb.ScopeMetrics{{Metrics: []*mpb.Metric{{
				Name: "http.duration",
				Unit: "ms",
				Data: &mpb.Metric_Histogram{Histogram: &mpb.Histogram{DataPoints: []*mpb.HistogramDataPoint{{
					TimeUnixNano:   ts,
					Count:          50,
					Sum:            &sum,
					BucketCounts:   []uint64{5, 20, 15, 10},
					ExplicitBounds: []float64{10, 50, 100},
					Attributes:     methodAttr,
				}}}},
			}}}},
		}}
		rows := convertMetrics(rm, loc)
		got := formatRow(rows[0], resourceAttrKeys, attrKeys)
		want := "2026-08-29T15:00:00Z\thttp.duration\thistogram\t" +
			`{"count":50,"sum":350.5,"bucket_counts":[5,20,15,10],"boundaries":[10,50,100]}` +
			"\tms\thost1\tGET"
		if got != want {
			t.Errorf("got:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("exponential_histogram", func(t *testing.T) {
		sum := 100.0
		rm := []*mpb.ResourceMetrics{{
			Resource: resource,
			ScopeMetrics: []*mpb.ScopeMetrics{{Metrics: []*mpb.Metric{{
				Name: "http.request.size",
				Unit: "By",
				Data: &mpb.Metric_ExponentialHistogram{ExponentialHistogram: &mpb.ExponentialHistogram{DataPoints: []*mpb.ExponentialHistogramDataPoint{{
					TimeUnixNano: ts,
					Count:        30,
					Sum:          &sum,
					ZeroCount:    2,
					Scale:        3,
					Positive: &mpb.ExponentialHistogramDataPoint_Buckets{
						Offset:       1,
						BucketCounts: []uint64{5, 10, 8},
					},
					Negative: &mpb.ExponentialHistogramDataPoint_Buckets{
						Offset:       0,
						BucketCounts: []uint64{3, 2},
					},
					Attributes: methodAttr,
				}}}},
			}}}},
		}}
		rows := convertMetrics(rm, loc)
		got := formatRow(rows[0], resourceAttrKeys, attrKeys)
		want := "2026-08-29T15:00:00Z\thttp.request.size\texponential_histogram\t" +
			`{"count":30,"sum":100,"zero_count":2,"scale":3,"positive":{"offset":1,"bucket_counts":[5,10,8]},"negative":{"offset":0,"bucket_counts":[3,2]}}` +
			"\tBy\thost1\tGET"
		if got != want {
			t.Errorf("got:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("summary", func(t *testing.T) {
		rm := []*mpb.ResourceMetrics{{
			Resource: resource,
			ScopeMetrics: []*mpb.ScopeMetrics{{Metrics: []*mpb.Metric{{
				Name: "rpc.duration",
				Unit: "ms",
				Data: &mpb.Metric_Summary{Summary: &mpb.Summary{DataPoints: []*mpb.SummaryDataPoint{{
					TimeUnixNano: ts,
					Count:        200,
					Sum:          1500.5,
					Attributes:   methodAttr,
				}}}},
			}}}},
		}}
		rows := convertMetrics(rm, loc)
		got := formatRow(rows[0], resourceAttrKeys, attrKeys)
		want := "2026-08-29T15:00:00Z\trpc.duration\tsummary\t1500.5\tms\thost1\tGET"
		if got != want {
			t.Errorf("got:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("missing_attributes_are_empty", func(t *testing.T) {
		rm := []*mpb.ResourceMetrics{{
			Resource: resource,
			ScopeMetrics: []*mpb.ScopeMetrics{{Metrics: []*mpb.Metric{{
				Name: "simple.metric",
				Unit: "",
				Data: &mpb.Metric_Gauge{Gauge: &mpb.Gauge{DataPoints: []*mpb.NumberDataPoint{{
					TimeUnixNano: ts,
					Value:        &mpb.NumberDataPoint_AsInt{AsInt: 1},
				}}}},
			}}}},
		}}
		rows := convertMetrics(rm, loc)
		got := formatRow(rows[0], resourceAttrKeys, attrKeys)
		want := "2026-08-29T15:00:00Z\tsimple.metric\tgauge\t1\t\thost1\t"
		if got != want {
			t.Errorf("got:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("timezone_affects_timestamp", func(t *testing.T) {
		jst := time.FixedZone("JST", 9*3600)
		// 2026-08-29 15:00:00 UTC = 2026-08-30 00:00:00 JST
		rm := []*mpb.ResourceMetrics{{
			Resource: resource,
			ScopeMetrics: []*mpb.ScopeMetrics{{Metrics: []*mpb.Metric{{
				Name: "cpu.usage",
				Unit: "%",
				Data: &mpb.Metric_Gauge{Gauge: &mpb.Gauge{DataPoints: []*mpb.NumberDataPoint{{
					TimeUnixNano: ts,
					Value:        &mpb.NumberDataPoint_AsDouble{AsDouble: 50},
					Attributes:   methodAttr,
				}}}},
			}}}},
		}}
		rows := convertMetrics(rm, jst)
		got := formatRow(rows[0], resourceAttrKeys, attrKeys)
		want := "2026-08-30T00:00:00+09:00\tcpu.usage\tgauge\t50\t%\thost1\tGET"
		if got != want {
			t.Errorf("got:\n%s\nwant:\n%s", got, want)
		}
	})
}
