package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	cpb "go.opentelemetry.io/proto/otlp/common/v1"
	mpb "go.opentelemetry.io/proto/otlp/metrics/v1"
	rpb "go.opentelemetry.io/proto/otlp/resource/v1"
)

type MetricRow struct {
	Timestamp          time.Time
	Name               string
	Type               string
	Value              string
	Unit               string
	ServiceNamespace   string
	ServiceName        string
	ResourceAttributes map[string]string
	Attributes         map[string]string
}

type HistogramValue struct {
	Count        uint64    `json:"count"`
	Sum          *float64  `json:"sum,omitempty"`
	Min          *float64  `json:"min,omitempty"`
	Max          *float64  `json:"max,omitempty"`
	BucketCounts []uint64  `json:"bucket_counts"`
	Boundaries   []float64 `json:"boundaries"`
}

type ExpHistogramValue struct {
	Count    uint64   `json:"count"`
	Sum      *float64 `json:"sum,omitempty"`
	Min      *float64 `json:"min,omitempty"`
	Max      *float64 `json:"max,omitempty"`
	ZeroCount uint64  `json:"zero_count"`
	Scale    int32    `json:"scale"`
	Positive *Buckets `json:"positive,omitempty"`
	Negative *Buckets `json:"negative,omitempty"`
}

type Buckets struct {
	Offset     int32    `json:"offset"`
	BucketCounts []uint64 `json:"bucket_counts"`
}

func extractAttributes(kvs []*cpb.KeyValue) map[string]string {
	attrs := make(map[string]string, len(kvs))
	for _, kv := range kvs {
		attrs[kv.Key] = anyValueToString(kv.Value)
	}
	return attrs
}

func anyValueToString(v *cpb.AnyValue) string {
	if v == nil {
		return ""
	}
	switch val := v.Value.(type) {
	case *cpb.AnyValue_StringValue:
		return val.StringValue
	case *cpb.AnyValue_IntValue:
		return fmt.Sprintf("%d", val.IntValue)
	case *cpb.AnyValue_DoubleValue:
		return fmt.Sprintf("%g", val.DoubleValue)
	case *cpb.AnyValue_BoolValue:
		return fmt.Sprintf("%t", val.BoolValue)
	case *cpb.AnyValue_BytesValue:
		return fmt.Sprintf("%x", val.BytesValue)
	default:
		return fmt.Sprintf("%v", v.Value)
	}
}

func extractServiceInfo(resource *rpb.Resource) (namespace, name string, allAttrs map[string]string) {
	allAttrs = make(map[string]string)
	if resource == nil {
		return "", "unknown", allAttrs
	}
	name = "unknown"
	for _, kv := range resource.Attributes {
		allAttrs[kv.Key] = anyValueToString(kv.Value)
		switch kv.Key {
		case "service.namespace":
			namespace = anyValueToString(kv.Value)
		case "service.name":
			name = anyValueToString(kv.Value)
		}
	}
	return namespace, name, allAttrs
}

func dataPointTimestamp(timeUnixNano uint64, loc *time.Location) time.Time {
	if timeUnixNano == 0 {
		return time.Now().In(loc)
	}
	return time.Unix(0, int64(timeUnixNano)).In(loc)
}

func dpAttributes(dp []*cpb.KeyValue) map[string]string {
	return extractAttributes(dp)
}

func convertMetrics(resourceMetrics []*mpb.ResourceMetrics, loc *time.Location) []MetricRow {
	var rows []MetricRow

	for _, rm := range resourceMetrics {
		ns, svcName, resAttrs := extractServiceInfo(rm.Resource)

		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				rows = append(rows, convertDataPoints(m, ns, svcName, resAttrs, loc)...)
			}
		}
	}
	return rows
}

func convertDataPoints(m *mpb.Metric, ns, svcName string, resAttrs map[string]string, loc *time.Location) []MetricRow {
	var rows []MetricRow
	unit := m.Unit

	switch data := m.Data.(type) {
	case *mpb.Metric_Gauge:
		for _, dp := range data.Gauge.DataPoints {
			rows = append(rows, MetricRow{
				Timestamp:          dataPointTimestamp(dp.TimeUnixNano, loc),
				Name:               m.Name,
				Type:               "gauge",
				Value:              numberValueToString(dp),
				Unit:               unit,
				ServiceNamespace:   ns,
				ServiceName:        svcName,
				ResourceAttributes: resAttrs,
				Attributes:         dpAttributes(dp.Attributes),
			})
		}
	case *mpb.Metric_Sum:
		for _, dp := range data.Sum.DataPoints {
			rows = append(rows, MetricRow{
				Timestamp:          dataPointTimestamp(dp.TimeUnixNano, loc),
				Name:               m.Name,
				Type:               "counter",
				Value:              numberValueToString(dp),
				Unit:               unit,
				ServiceNamespace:   ns,
				ServiceName:        svcName,
				ResourceAttributes: resAttrs,
				Attributes:         dpAttributes(dp.Attributes),
			})
		}
	case *mpb.Metric_Histogram:
		for _, dp := range data.Histogram.DataPoints {
			hv := HistogramValue{
				Count:        dp.Count,
				BucketCounts: dp.BucketCounts,
				Boundaries:   dp.ExplicitBounds,
			}
			if dp.Sum != nil {
				hv.Sum = dp.Sum
			}
			if dp.Min != nil {
				hv.Min = dp.Min
			}
			if dp.Max != nil {
				hv.Max = dp.Max
			}
			jsonBytes, _ := json.Marshal(hv)
			rows = append(rows, MetricRow{
				Timestamp:          dataPointTimestamp(dp.TimeUnixNano, loc),
				Name:               m.Name,
				Type:               "histogram",
				Value:              string(jsonBytes),
				Unit:               unit,
				ServiceNamespace:   ns,
				ServiceName:        svcName,
				ResourceAttributes: resAttrs,
				Attributes:         dpAttributes(dp.Attributes),
			})
		}
	case *mpb.Metric_ExponentialHistogram:
		for _, dp := range data.ExponentialHistogram.DataPoints {
			ev := ExpHistogramValue{
				Count:     dp.Count,
				ZeroCount: dp.ZeroCount,
				Scale:     dp.Scale,
			}
			if dp.Sum != nil {
				ev.Sum = dp.Sum
			}
			if dp.Min != nil {
				ev.Min = dp.Min
			}
			if dp.Max != nil {
				ev.Max = dp.Max
			}
			if dp.Positive != nil {
				ev.Positive = &Buckets{
					Offset:       dp.Positive.Offset,
					BucketCounts: dp.Positive.BucketCounts,
				}
			}
			if dp.Negative != nil {
				ev.Negative = &Buckets{
					Offset:       dp.Negative.Offset,
					BucketCounts: dp.Negative.BucketCounts,
				}
			}
			jsonBytes, _ := json.Marshal(ev)
			rows = append(rows, MetricRow{
				Timestamp:          dataPointTimestamp(dp.TimeUnixNano, loc),
				Name:               m.Name,
				Type:               "exponential_histogram",
				Value:              string(jsonBytes),
				Unit:               unit,
				ServiceNamespace:   ns,
				ServiceName:        svcName,
				ResourceAttributes: resAttrs,
				Attributes:         dpAttributes(dp.Attributes),
			})
		}
	case *mpb.Metric_Summary:
		for _, dp := range data.Summary.DataPoints {
			rows = append(rows, MetricRow{
				Timestamp:          dataPointTimestamp(dp.TimeUnixNano, loc),
				Name:               m.Name,
				Type:               "summary",
				Value:              fmt.Sprintf("%g", dp.Sum),
				Unit:               unit,
				ServiceNamespace:   ns,
				ServiceName:        svcName,
				ResourceAttributes: resAttrs,
				Attributes:         dpAttributes(dp.Attributes),
			})
		}
	}
	return rows
}

func numberValueToString(dp *mpb.NumberDataPoint) string {
	switch v := dp.Value.(type) {
	case *mpb.NumberDataPoint_AsInt:
		return fmt.Sprintf("%d", v.AsInt)
	case *mpb.NumberDataPoint_AsDouble:
		return fmt.Sprintf("%g", v.AsDouble)
	default:
		return "0"
	}
}

func formatRow(row MetricRow, resourceAttrKeys, attrKeys []string) string {
	fields := []string{
		row.Timestamp.Format(time.RFC3339),
		row.Name,
		row.Type,
		row.Value,
		row.Unit,
	}
	for _, k := range resourceAttrKeys {
		fields = append(fields, row.ResourceAttributes[k])
	}
	for _, k := range attrKeys {
		fields = append(fields, row.Attributes[k])
	}
	return strings.Join(fields, "\t")
}

func headerLine(resourceAttrKeys, attrKeys []string) string {
	fields := []string{"timestamp", "name", "type", "value", "unit"}
	for _, k := range resourceAttrKeys {
		fields = append(fields, "resource."+k)
	}
	for _, k := range attrKeys {
		fields = append(fields, k)
	}
	return strings.Join(fields, "\t")
}
