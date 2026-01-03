// SPDX-License-Identifier: MIT

package tailer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestProcessFile(t *testing.T) {
	// Create temp file
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	content := "line1\nline2\nline3\nline4\nline5\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var lines []string
	handler := func(line string) {
		lines = append(lines, line)
	}

	count, err := ProcessFile(path, handler, 0)
	if err != nil {
		t.Fatalf("ProcessFile() error = %v", err)
	}

	if count != 5 {
		t.Errorf("ProcessFile() count = %d, want 5", count)
	}

	if len(lines) != 5 {
		t.Errorf("len(lines) = %d, want 5", len(lines))
	}

	expected := []string{"line1", "line2", "line3", "line4", "line5"}
	for i, exp := range expected {
		if lines[i] != exp {
			t.Errorf("lines[%d] = %q, want %q", i, lines[i], exp)
		}
	}
}

func TestProcessFile_WithLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	content := "line1\nline2\nline3\nline4\nline5\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var lines []string
	handler := func(line string) {
		lines = append(lines, line)
	}

	count, err := ProcessFile(path, handler, 3)
	if err != nil {
		t.Fatalf("ProcessFile() error = %v", err)
	}

	if count != 3 {
		t.Errorf("ProcessFile() count = %d, want 3", count)
	}

	if len(lines) != 3 {
		t.Errorf("len(lines) = %d, want 3", len(lines))
	}
}

func TestProcessFile_NonExistent(t *testing.T) {
	_, err := ProcessFile("/nonexistent/file.log", func(string) {}, 0)
	if err == nil {
		t.Error("ProcessFile() should return error for non-existent file")
	}
}

func TestProcessReader(t *testing.T) {
	content := "alpha\nbeta\ngamma\n"
	reader := strings.NewReader(content)

	var lines []string
	handler := func(line string) {
		lines = append(lines, line)
	}

	count, err := ProcessReader(reader, handler, 0)
	if err != nil {
		t.Fatalf("ProcessReader() error = %v", err)
	}

	if count != 3 {
		t.Errorf("ProcessReader() count = %d, want 3", count)
	}

	expected := []string{"alpha", "beta", "gamma"}
	for i, exp := range expected {
		if lines[i] != exp {
			t.Errorf("lines[%d] = %q, want %q", i, lines[i], exp)
		}
	}
}

func TestProcessReader_NoTrailingNewline(t *testing.T) {
	content := "line1\nline2\nline3" // No trailing newline
	reader := strings.NewReader(content)

	var lines []string
	handler := func(line string) {
		lines = append(lines, line)
	}

	count, err := ProcessReader(reader, handler, 0)
	if err != nil {
		t.Fatalf("ProcessReader() error = %v", err)
	}

	if count != 3 {
		t.Errorf("ProcessReader() count = %d, want 3", count)
	}

	if lines[2] != "line3" {
		t.Errorf("lines[2] = %q, want %q", lines[2], "line3")
	}
}

func TestTailer_StartFromBeginning(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	content := "line1\nline2\nline3\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var mu sync.Mutex
	var lines []string
	handler := func(line string) {
		mu.Lock()
		lines = append(lines, line)
		mu.Unlock()
	}

	tailer := New(path, handler, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := tailer.StartFromBeginning(ctx); err != nil {
		t.Fatalf("StartFromBeginning() error = %v", err)
	}

	// Wait for lines to be processed
	time.Sleep(100 * time.Millisecond)

	if err := tailer.Stop(); err != nil {
		t.Errorf("Stop() error = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(lines) != 3 {
		t.Errorf("len(lines) = %d, want 3", len(lines))
	}
}

func TestTailer_FollowNewLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	// Create empty file
	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var mu sync.Mutex
	var lines []string
	handler := func(line string) {
		mu.Lock()
		lines = append(lines, line)
		mu.Unlock()
	}

	tailer := New(path, handler, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := tailer.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Give the tailer time to start watching
	time.Sleep(100 * time.Millisecond)

	// Append lines
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}

	if _, err := f.WriteString("new line 1\n"); err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}
	if _, err := f.WriteString("new line 2\n"); err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}
	f.Sync() // Ensure writes are flushed
	f.Close()

	// Wait for lines to be processed
	time.Sleep(500 * time.Millisecond)

	if err := tailer.Stop(); err != nil {
		t.Errorf("Stop() error = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(lines) != 2 {
		t.Errorf("len(lines) = %d, want 2", len(lines))
	}
}

func TestTailer_NonExistentFile(t *testing.T) {
	tailer := New("/nonexistent/file.log", func(string) {}, nil)

	err := tailer.Start(context.Background())
	if err == nil {
		t.Error("Start() should return error for non-existent file")
	}
}

func TestTailer_DoubleStart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	if err := os.WriteFile(path, []byte("test\n"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	tailer := New(path, func(string) {}, nil)

	ctx := context.Background()

	if err := tailer.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer tailer.Stop()

	err := tailer.Start(ctx)
	if err == nil {
		t.Error("Second Start() should return error")
	}
}

func TestTailer_Path(t *testing.T) {
	tailer := New("/var/log/test.log", func(string) {}, nil)
	if tailer.Path() != "/var/log/test.log" {
		t.Errorf("Path() = %q, want %q", tailer.Path(), "/var/log/test.log")
	}
}

func TestLineScanner_LongLines(t *testing.T) {
	// Create a line that's 100KB
	longLine := strings.Repeat("x", 100*1024)
	content := longLine + "\nshort line\n"
	reader := strings.NewReader(content)

	var lines []string
	handler := func(line string) {
		lines = append(lines, line)
	}

	count, err := ProcessReader(reader, handler, 0)
	if err != nil {
		t.Fatalf("ProcessReader() error = %v", err)
	}

	if count != 2 {
		t.Errorf("ProcessReader() count = %d, want 2", count)
	}

	if len(lines[0]) != 100*1024 {
		t.Errorf("len(lines[0]) = %d, want %d", len(lines[0]), 100*1024)
	}
}

func TestProcessFile_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.log")

	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var lines []string
	handler := func(line string) {
		lines = append(lines, line)
	}

	count, err := ProcessFile(path, handler, 0)
	if err != nil {
		t.Fatalf("ProcessFile() error = %v", err)
	}

	if count != 0 {
		t.Errorf("ProcessFile() count = %d, want 0", count)
	}

	if len(lines) != 0 {
		t.Errorf("len(lines) = %d, want 0", len(lines))
	}
}

func TestProcessReader_EmptyLines(t *testing.T) {
	content := "line1\n\nline3\n\n"
	reader := strings.NewReader(content)

	var lines []string
	handler := func(line string) {
		lines = append(lines, line)
	}

	count, err := ProcessReader(reader, handler, 0)
	if err != nil {
		t.Fatalf("ProcessReader() error = %v", err)
	}

	if count != 4 {
		t.Errorf("ProcessReader() count = %d, want 4", count)
	}

	expected := []string{"line1", "", "line3", ""}
	for i, exp := range expected {
		if i >= len(lines) {
			t.Errorf("missing line at index %d", i)
			continue
		}
		if lines[i] != exp {
			t.Errorf("lines[%d] = %q, want %q", i, lines[i], exp)
		}
	}
}
