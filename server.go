package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	collpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	"google.golang.org/protobuf/proto"
)

type Server struct {
	writer     *TSVWriter
	loc        *time.Location
	httpServer *http.Server
}

func NewServer(addr string, writer *TSVWriter, loc *time.Location) *Server {
	s := &Server{
		writer: writer,
		loc:    loc,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/metrics", s.handleMetrics)
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	return s
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("read body: %v", err), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	req := &collpb.ExportMetricsServiceRequest{}
	if err := proto.Unmarshal(body, req); err != nil {
		http.Error(w, fmt.Sprintf("unmarshal: %v", err), http.StatusBadRequest)
		return
	}

	rows := convertMetrics(req.ResourceMetrics, s.loc)
	if err := s.writer.WriteRows(rows); err != nil {
		log.Printf("write error: %v", err)
		http.Error(w, fmt.Sprintf("write: %v", err), http.StatusInternalServerError)
		return
	}

	resp := &collpb.ExportMetricsServiceResponse{}
	respBytes, err := proto.Marshal(resp)
	if err != nil {
		http.Error(w, fmt.Sprintf("marshal response: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-protobuf")
	w.WriteHeader(http.StatusOK)
	w.Write(respBytes)
}

func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
