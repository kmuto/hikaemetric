package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

var safeNameRe = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

func sanitizeFileName(s string) string {
	return safeNameRe.ReplaceAllString(s, "")
}

func buildFileName(ns, svcName, date string) string {
	svc := sanitizeFileName(svcName)
	if svc == "" {
		svc = "unknown"
	}
	nsClean := sanitizeFileName(ns)
	if nsClean != "" {
		return fmt.Sprintf("%s-%s-%s.tsv", nsClean, svc, date)
	}
	return fmt.Sprintf("%s-%s.tsv", svc, date)
}

type fileEntry struct {
	file       *os.File
	writer     *bufio.Writer
	lastAccess time.Time
}

type TSVWriter struct {
	mu               sync.Mutex
	outputDir        string
	resourceAttrKeys []string
	attrKeys         []string
	loc              *time.Location
	files            map[string]*fileEntry
	idleTimeout      time.Duration
	stopCh           chan struct{}
	wg               sync.WaitGroup
}

func NewTSVWriter(outputDir string, resourceAttrKeys, attrKeys []string, loc *time.Location) *TSVWriter {
	w := &TSVWriter{
		outputDir:        outputDir,
		resourceAttrKeys: resourceAttrKeys,
		attrKeys:         attrKeys,
		loc:              loc,
		files:            make(map[string]*fileEntry),
		idleTimeout:      5 * time.Minute,
		stopCh:           make(chan struct{}),
	}
	w.wg.Add(1)
	go w.reaper()
	return w
}

func (w *TSVWriter) reaper() {
	defer w.wg.Done()
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			w.closeIdle()
		case <-w.stopCh:
			return
		}
	}
}

func (w *TSVWriter) closeIdle() {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := time.Now()
	for key, entry := range w.files {
		if now.Sub(entry.lastAccess) > w.idleTimeout {
			entry.writer.Flush()
			entry.file.Close()
			delete(w.files, key)
		}
	}
}

func (w *TSVWriter) getFile(ns, svcName string, ts time.Time) (*fileEntry, error) {
	date := ts.In(w.loc).Format("20060102")
	name := buildFileName(ns, svcName, date)
	key := name

	if entry, ok := w.files[key]; ok {
		entry.lastAccess = time.Now()
		return entry, nil
	}

	path := filepath.Join(w.outputDir, name)
	needHeader := false
	if _, err := os.Stat(path); os.IsNotExist(err) {
		needHeader = true
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}

	bw := bufio.NewWriter(f)
	if needHeader {
		header := headerLine(w.resourceAttrKeys, w.attrKeys)
		fmt.Fprintln(bw, header)
	}

	entry := &fileEntry{
		file:       f,
		writer:     bw,
		lastAccess: time.Now(),
	}
	w.files[key] = entry
	return entry, nil
}

func (w *TSVWriter) WriteRows(rows []MetricRow) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	for _, row := range rows {
		entry, err := w.getFile(row.ServiceNamespace, row.ServiceName, row.Timestamp)
		if err != nil {
			return err
		}
		line := formatRow(row, w.resourceAttrKeys, w.attrKeys)
		fmt.Fprintln(entry.writer, line)
	}
	return nil
}

func (w *TSVWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, entry := range w.files {
		entry.writer.Flush()
	}
}

func (w *TSVWriter) Close() {
	close(w.stopCh)
	w.wg.Wait()

	w.mu.Lock()
	defer w.mu.Unlock()
	for key, entry := range w.files {
		entry.writer.Flush()
		entry.file.Close()
		delete(w.files, key)
	}
}
