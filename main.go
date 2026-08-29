package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	port := flag.Int("port", 14318, "listen port for OTLP HTTP receiver")
	outputDir := flag.String("output", ".", "output directory for TSV files")
	resourceAttrs := flag.String("resource-attrs", "", "comma-separated resource attribute keys to include")
	attrs := flag.String("attrs", "", "comma-separated attribute keys to include")
	timezone := flag.String("timezone", "", "timezone for timestamps and file rotation (e.g. Asia/Tokyo). defaults to system local")
	flag.Parse()

	loc := time.Local
	if *timezone != "" {
		var err error
		loc, err = time.LoadLocation(*timezone)
		if err != nil {
			log.Fatalf("invalid timezone %q: %v", *timezone, err)
		}
	}

	var resourceAttrKeys, attrKeys []string
	if *resourceAttrs != "" {
		resourceAttrKeys = strings.Split(*resourceAttrs, ",")
	}
	if *attrs != "" {
		attrKeys = strings.Split(*attrs, ",")
	}

	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("create output dir: %v", err)
	}

	writer := NewTSVWriter(*outputDir, resourceAttrKeys, attrKeys, loc)

	addr := fmt.Sprintf(":%d", *port)
	srv := NewServer(addr, writer, loc)

	go func() {
		log.Printf("listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
	writer.Flush()
	writer.Close()
	log.Println("done")
}
