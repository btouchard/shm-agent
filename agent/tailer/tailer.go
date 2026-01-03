// SPDX-License-Identifier: MIT

// Package tailer provides file tailing with rotation support.
package tailer

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"

	"github.com/nxadm/tail"
)

// LineHandler is called for each line read from the file.
type LineHandler func(line string)

// Tailer watches and tails a file.
type Tailer struct {
	path    string
	handler LineHandler
	logger  *slog.Logger

	mu     sync.Mutex
	tail   *tail.Tail
	cancel context.CancelFunc
}

// New creates a new Tailer for the given file path.
func New(path string, handler LineHandler, logger *slog.Logger) *Tailer {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	return &Tailer{
		path:    path,
		handler: handler,
		logger:  logger,
	}
}

// Start begins tailing the file.
// It starts from the end of the file and follows new lines.
func (t *Tailer) Start(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.tail != nil {
		return fmt.Errorf("tailer already running")
	}

	// Check if file exists
	if _, err := os.Stat(t.path); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", t.path)
	}

	cfg := tail.Config{
		Follow:    true,
		ReOpen:    true, // Handle log rotation
		MustExist: true,
		Location:  &tail.SeekInfo{Offset: 0, Whence: io.SeekEnd}, // Start at end
		Logger:    tail.DiscardingLogger,
	}

	tailFile, err := tail.TailFile(t.path, cfg)
	if err != nil {
		return fmt.Errorf("tailing file: %w", err)
	}

	t.tail = tailFile

	ctx, cancel := context.WithCancel(ctx)
	t.cancel = cancel

	go t.run(ctx)

	t.logger.Info("started tailing file", "path", t.path)
	return nil
}

// StartFromBeginning begins tailing from the beginning of the file.
// Useful for testing and one-shot processing.
func (t *Tailer) StartFromBeginning(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.tail != nil {
		return fmt.Errorf("tailer already running")
	}

	if _, err := os.Stat(t.path); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", t.path)
	}

	cfg := tail.Config{
		Follow:    true,
		ReOpen:    true,
		MustExist: true,
		Location:  &tail.SeekInfo{Offset: 0, Whence: io.SeekStart}, // Start at beginning
		Logger:    tail.DiscardingLogger,
	}

	tailFile, err := tail.TailFile(t.path, cfg)
	if err != nil {
		return fmt.Errorf("tailing file: %w", err)
	}

	t.tail = tailFile

	ctx, cancel := context.WithCancel(ctx)
	t.cancel = cancel

	go t.run(ctx)

	t.logger.Info("started tailing file from beginning", "path", t.path)
	return nil
}

// run processes lines from the tail.
func (t *Tailer) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case line, ok := <-t.tail.Lines:
			if !ok {
				t.logger.Debug("tail channel closed", "path", t.path)
				return
			}
			if line.Err != nil {
				t.logger.Error("error reading line", "path", t.path, "error", line.Err)
				continue
			}
			if t.handler != nil {
				t.handler(line.Text)
			}
		}
	}
}

// Stop stops tailing the file.
func (t *Tailer) Stop() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.cancel != nil {
		t.cancel()
		t.cancel = nil
	}

	if t.tail != nil {
		err := t.tail.Stop()
		t.tail.Cleanup()
		t.tail = nil
		t.logger.Info("stopped tailing file", "path", t.path)
		return err
	}

	return nil
}

// Path returns the file path being tailed.
func (t *Tailer) Path() string {
	return t.path
}

// ProcessFile reads an entire file and processes each line.
// This is a one-shot operation, not continuous tailing.
// Useful for testing and batch processing.
func ProcessFile(path string, handler LineHandler, limit int) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("opening file: %w", err)
	}
	defer file.Close()

	return ProcessReader(file, handler, limit)
}

// ProcessReader reads from a reader and processes each line.
func ProcessReader(r io.Reader, handler LineHandler, limit int) (int, error) {
	scanner := NewLineScanner(r)
	count := 0

	for scanner.Scan() {
		if limit > 0 && count >= limit {
			break
		}
		handler(scanner.Text())
		count++
	}

	if err := scanner.Err(); err != nil {
		return count, fmt.Errorf("reading: %w", err)
	}

	return count, nil
}

// LineScanner wraps bufio.Scanner with a larger buffer for long lines.
type LineScanner struct {
	buf    []byte
	reader io.Reader
	line   string
	err    error
}

// NewLineScanner creates a scanner with a large buffer.
func NewLineScanner(r io.Reader) *LineScanner {
	return &LineScanner{
		buf:    make([]byte, 0, 64*1024), // 64KB buffer
		reader: r,
	}
}

// Scan reads the next line.
func (s *LineScanner) Scan() bool {
	s.line = ""
	s.buf = s.buf[:0]

	for {
		b := make([]byte, 1)
		n, err := s.reader.Read(b)
		if n > 0 {
			if b[0] == '\n' {
				s.line = string(s.buf)
				return true
			}
			s.buf = append(s.buf, b[0])

			// Limit line length to prevent memory issues
			if len(s.buf) > 1024*1024 { // 1MB max line
				s.err = fmt.Errorf("line too long")
				return false
			}
		}
		if err != nil {
			if err == io.EOF {
				if len(s.buf) > 0 {
					s.line = string(s.buf)
					return true
				}
				return false
			}
			s.err = err
			return false
		}
	}
}

// Text returns the current line.
func (s *LineScanner) Text() string {
	return s.line
}

// Err returns any error from scanning.
func (s *LineScanner) Err() error {
	return s.err
}
