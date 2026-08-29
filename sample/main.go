package main

import (
	"bytes"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	cpb "go.opentelemetry.io/proto/otlp/common/v1"
	collpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	mpb "go.opentelemetry.io/proto/otlp/metrics/v1"
	rpb "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/protobuf/proto"
)

var (
	endpoint = "http://localhost:14318/v1/metrics"
	services = []struct {
		namespace string
		name      string
		host      string
	}{
		{"production", "web", "web-01"},
		{"production", "api", "api-01"},
		{"", "worker", "worker-01"},
	}
	methods    = []string{"GET", "POST", "PUT", "DELETE"}
	statusCodes = []string{"200", "201", "400", "404", "500"}
)

func main() {
	log.Println("sending random metrics to", endpoint)
	log.Println("press Ctrl-C to stop")

	for {
		for _, svc := range services {
			req := buildRequest(svc.namespace, svc.name, svc.host)
			if err := send(req); err != nil {
				log.Printf("send error: %v", err)
			}
		}
		wait := time.Duration(1+rand.Intn(3)) * time.Second
		log.Printf("sent batch, next in %s", wait)
		time.Sleep(wait)
	}
}

func send(req *collpb.ExportMetricsServiceRequest) error {
	body, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	resp, err := http.Post(endpoint, "application/x-protobuf", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("post: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

func buildRequest(namespace, svcName, host string) *collpb.ExportMetricsServiceRequest {
	now := uint64(time.Now().UnixNano())

	resAttrs := []*cpb.KeyValue{
		strKV("service.name", svcName),
		strKV("host.name", host),
	}
	if namespace != "" {
		resAttrs = append(resAttrs, strKV("service.namespace", namespace))
	}

	resource := &rpb.Resource{Attributes: resAttrs}

	metrics := []*mpb.Metric{
		gaugeDouble(now),
		gaugeInt(now),
		counter(now),
		histogram(now),
		exponentialHistogram(now),
		summary(now),
	}

	return &collpb.ExportMetricsServiceRequest{
		ResourceMetrics: []*mpb.ResourceMetrics{{
			Resource: resource,
			ScopeMetrics: []*mpb.ScopeMetrics{{
				Scope:   &cpb.InstrumentationScope{Name: "sample-sender", Version: "0.1.0"},
				Metrics: metrics,
			}},
		}},
	}
}

func gaugeDouble(ts uint64) *mpb.Metric {
	return &mpb.Metric{
		Name: "system.cpu.usage",
		Unit: "%",
		Data: &mpb.Metric_Gauge{Gauge: &mpb.Gauge{
			DataPoints: []*mpb.NumberDataPoint{{
				TimeUnixNano: ts,
				Value:        &mpb.NumberDataPoint_AsDouble{AsDouble: 20 + rand.Float64()*80},
				Attributes:   []*cpb.KeyValue{strKV("core", fmt.Sprintf("%d", rand.Intn(8)))},
			}},
		}},
	}
}

func gaugeInt(ts uint64) *mpb.Metric {
	return &mpb.Metric{
		Name: "runtime.goroutines",
		Unit: "1",
		Data: &mpb.Metric_Gauge{Gauge: &mpb.Gauge{
			DataPoints: []*mpb.NumberDataPoint{{
				TimeUnixNano: ts,
				Value:        &mpb.NumberDataPoint_AsInt{AsInt: int64(10 + rand.Intn(200))},
			}},
		}},
	}
}

func counter(ts uint64) *mpb.Metric {
	return &mpb.Metric{
		Name: "http.requests.total",
		Unit: "1",
		Data: &mpb.Metric_Sum{Sum: &mpb.Sum{
			IsMonotonic:            true,
			AggregationTemporality: mpb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
			DataPoints: []*mpb.NumberDataPoint{{
				TimeUnixNano: ts,
				Value:        &mpb.NumberDataPoint_AsInt{AsInt: int64(rand.Intn(10000))},
				Attributes: []*cpb.KeyValue{
					strKV("method", methods[rand.Intn(len(methods))]),
					strKV("status_code", statusCodes[rand.Intn(len(statusCodes))]),
				},
			}},
		}},
	}
}

func histogram(ts uint64) *mpb.Metric {
	counts := make([]uint64, 6)
	for i := range counts {
		counts[i] = uint64(rand.Intn(50))
	}
	sum := 50 + rand.Float64()*500

	return &mpb.Metric{
		Name: "http.request.duration",
		Unit: "ms",
		Data: &mpb.Metric_Histogram{Histogram: &mpb.Histogram{
			AggregationTemporality: mpb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
			DataPoints: []*mpb.HistogramDataPoint{{
				TimeUnixNano:   ts,
				Count:          sumUint64(counts),
				Sum:            &sum,
				BucketCounts:   counts,
				ExplicitBounds: []float64{5, 10, 25, 50, 100},
				Attributes: []*cpb.KeyValue{
					strKV("method", methods[rand.Intn(len(methods))]),
				},
			}},
		}},
	}
}

func exponentialHistogram(ts uint64) *mpb.Metric {
	posCounts := make([]uint64, 4)
	for i := range posCounts {
		posCounts[i] = uint64(rand.Intn(20))
	}
	sum := 10 + rand.Float64()*100

	return &mpb.Metric{
		Name: "http.request.size",
		Unit: "By",
		Data: &mpb.Metric_ExponentialHistogram{ExponentialHistogram: &mpb.ExponentialHistogram{
			AggregationTemporality: mpb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
			DataPoints: []*mpb.ExponentialHistogramDataPoint{{
				TimeUnixNano: ts,
				Count:        sumUint64(posCounts) + uint64(rand.Intn(5)),
				Sum:          &sum,
				ZeroCount:    uint64(rand.Intn(5)),
				Scale:        3,
				Positive: &mpb.ExponentialHistogramDataPoint_Buckets{
					Offset:       1,
					BucketCounts: posCounts,
				},
				Negative: &mpb.ExponentialHistogramDataPoint_Buckets{
					Offset:       0,
					BucketCounts: []uint64{uint64(rand.Intn(5)), uint64(rand.Intn(5))},
				},
			}},
		}},
	}
}

func summary(ts uint64) *mpb.Metric {
	count := uint64(50 + rand.Intn(200))
	sum := float64(count) * (10 + rand.Float64()*50)

	return &mpb.Metric{
		Name: "rpc.server.duration",
		Unit: "ms",
		Data: &mpb.Metric_Summary{Summary: &mpb.Summary{
			DataPoints: []*mpb.SummaryDataPoint{{
				TimeUnixNano:   ts,
				Count:          count,
				Sum:            sum,
				QuantileValues: []*mpb.SummaryDataPoint_ValueAtQuantile{
					{Quantile: 0.5, Value: 10 + rand.Float64()*20},
					{Quantile: 0.9, Value: 30 + rand.Float64()*30},
					{Quantile: 0.99, Value: 60 + rand.Float64()*40},
				},
			}},
		}},
	}
}

func strKV(key, val string) *cpb.KeyValue {
	return &cpb.KeyValue{
		Key:   key,
		Value: &cpb.AnyValue{Value: &cpb.AnyValue_StringValue{StringValue: val}},
	}
}

func sumUint64(vals []uint64) uint64 {
	var s uint64
	for _, v := range vals {
		s += v
	}
	return s
}
