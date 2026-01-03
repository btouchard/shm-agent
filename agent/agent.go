// SPDX-License-Identifier: MIT

// Package agent provides the main orchestration for shm-agent.
package agent

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/kolapsis/shm-agent/agent/aggregator"
	"github.com/kolapsis/shm-agent/agent/config"
	"github.com/kolapsis/shm-agent/agent/identity"
	"github.com/kolapsis/shm-agent/agent/matcher"
	"github.com/kolapsis/shm-agent/agent/parser"
	"github.com/kolapsis/shm-agent/agent/sender"
	"github.com/kolapsis/shm-agent/agent/tailer"
)

// Agent orchestrates log collection and metric aggregation.
type Agent struct {
	cfg        *config.Config
	logger     *slog.Logger
	aggregator *aggregator.Aggregator
	sender     *sender.Sender
	tailers    []*tailer.Tailer
	processors []*sourceProcessor
	dryRun     bool
	verbosity  int

	mu          sync.Mutex
	running     bool
	startTime   time.Time
	linesParsed atomic.Int64
	linesErrors atomic.Int64
}

// sourceProcessor processes lines from a single source.
type sourceProcessor struct {
	source     *config.Source
	parser     parser.Parser
	metrics    []*metricProcessor
	aggregator *aggregator.Aggregator
	logger     *slog.Logger
	verbosity  int

	linesParsed  atomic.Int64
	linesMatched atomic.Int64
	parseErrors  atomic.Int64
}

// metricProcessor processes a single metric configuration.
type metricProcessor struct {
	cfg     *config.Metric
	matcher *matcher.Matcher
}

// Options configures the agent.
type Options struct {
	Config    *config.Config
	Logger    *slog.Logger
	DryRun    bool
	Verbosity int // 0=errors, 1=matches, 2=all lines
}

// New creates a new Agent.
func New(opts Options) (*Agent, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	agg := aggregator.New()

	// Initialize processors for each source
	var processors []*sourceProcessor
	for i := range opts.Config.Sources {
		src := &opts.Config.Sources[i]
		proc, err := newSourceProcessor(src, agg, logger, opts.Verbosity)
		if err != nil {
			return nil, fmt.Errorf("source %s: %w", src.Path, err)
		}
		processors = append(processors, proc)
	}

	return &Agent{
		cfg:        opts.Config,
		logger:     logger,
		aggregator: agg,
		processors: processors,
		dryRun:     opts.DryRun,
		verbosity:  opts.Verbosity,
	}, nil
}

// newSourceProcessor creates a processor for a source.
func newSourceProcessor(src *config.Source, agg *aggregator.Aggregator, logger *slog.Logger, verbosity int) (*sourceProcessor, error) {
	p, err := parser.New(src.Format, src.Pattern)
	if err != nil {
		return nil, fmt.Errorf("creating parser: %w", err)
	}

	var metrics []*metricProcessor
	for i := range src.Metrics {
		m := &src.Metrics[i]

		// Register metric with aggregator
		agg.Register(m.Name, aggregator.MetricType(m.Type))

		// Create matcher
		match, err := matcher.New(m.Match)
		if err != nil {
			return nil, fmt.Errorf("metric %s: %w", m.Name, err)
		}

		metrics = append(metrics, &metricProcessor{
			cfg:     m,
			matcher: match,
		})
	}

	return &sourceProcessor{
		source:     src,
		parser:     p,
		metrics:    metrics,
		aggregator: agg,
		logger:     logger,
		verbosity:  verbosity,
	}, nil
}

// Run starts the agent and blocks until stopped.
func (a *Agent) Run(ctx context.Context) error {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return fmt.Errorf("agent already running")
	}
	a.running = true
	a.startTime = time.Now()
	a.mu.Unlock()

	// Load or generate identity
	ident, err := identity.LoadOrGenerate(a.cfg.IdentityFile)
	if err != nil {
		return fmt.Errorf("loading identity: %w", err)
	}
	a.logger.Info("loaded identity", "instance_id", ident.InstanceID, "identity_file", a.cfg.IdentityFile)

	// Create sender (unless dry-run)
	if !a.dryRun {
		a.sender = sender.New(sender.Config{
			ServerURL:   a.cfg.ServerURL,
			AppName:     a.cfg.AppName,
			AppVersion:  a.cfg.AppVersion,
			Environment: a.cfg.Environment,
			Identity:    ident,
			Logger:      a.logger,
		})

		// Register with server
		if err := a.sender.Register(ctx); err != nil {
			return fmt.Errorf("registering with server: %w", err)
		}
	}

	// Start tailers
	for _, proc := range a.processors {
		t := tailer.New(proc.source.Path, proc.processLine, a.logger)
		if err := t.Start(ctx); err != nil {
			a.stopTailers()
			return fmt.Errorf("starting tailer for %s: %w", proc.source.Path, err)
		}
		a.tailers = append(a.tailers, t)
	}

	// Setup signal handlers
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGUSR1)

	// Start snapshot ticker
	ticker := time.NewTicker(a.cfg.Interval)
	defer ticker.Stop()

	a.logger.Info("agent started",
		"interval", a.cfg.Interval,
		"sources", len(a.processors),
		"dry_run", a.dryRun,
	)

	for {
		select {
		case <-ctx.Done():
			a.logger.Info("shutting down...")
			a.stopTailers()
			return nil

		case sig := <-sigChan:
			switch sig {
			case syscall.SIGUSR1:
				a.logger.Info("received SIGUSR1, dumping metrics")
				a.dumpMetrics()
			case syscall.SIGTERM, syscall.SIGINT:
				a.logger.Info("received shutdown signal")
				a.stopTailers()
				return nil
			}

		case <-ticker.C:
			if err := a.sendSnapshot(ctx); err != nil {
				a.logger.Error("failed to send snapshot", "error", err)
			}
		}
	}
}

// processLine processes a single log line.
func (p *sourceProcessor) processLine(line string) {
	if p.verbosity >= 2 {
		p.logger.Debug("processing line", "line", line)
	}

	// Parse the line
	data := p.parser.Parse(line)
	if data == nil {
		p.parseErrors.Add(1)
		if p.verbosity >= 1 {
			p.logger.Debug("failed to parse line", "line", line)
		}
		return
	}

	p.linesParsed.Add(1)

	// Process each metric
	for _, m := range p.metrics {
		if !m.matcher.Match(data) {
			continue
		}

		p.linesMatched.Add(1)

		if p.verbosity >= 1 {
			p.logger.Debug("matched metric", "metric", m.cfg.Name, "type", m.cfg.Type)
		}

		switch m.cfg.Type {
		case "counter":
			p.aggregator.Inc(m.cfg.Name)

		case "gauge":
			if m.cfg.Extract != nil {
				if val, ok := parser.GetFieldFloat(data, m.cfg.Extract.Field); ok {
					p.aggregator.SetGauge(m.cfg.Name, val)
				}
			}

		case "sum":
			if m.cfg.Extract != nil {
				if val, ok := parser.GetFieldFloat(data, m.cfg.Extract.Field); ok {
					p.aggregator.Add(m.cfg.Name, val)
				}
			}

		case "set":
			if m.cfg.Extract != nil {
				if val, ok := parser.GetFieldString(data, m.cfg.Extract.Field); ok {
					p.aggregator.AddToSet(m.cfg.Name, val)
				}
			}
		}
	}
}

// sendSnapshot sends the current metrics.
func (a *Agent) sendSnapshot(ctx context.Context) error {
	metrics := a.aggregator.Snapshot()

	if a.dryRun {
		a.printDryRunSnapshot(metrics)
		return nil
	}

	if a.sender != nil {
		return a.sender.SendSnapshot(ctx, metrics)
	}

	return nil
}

// dumpMetrics prints current metrics without reset (for SIGUSR1).
func (a *Agent) dumpMetrics() {
	metrics := a.aggregator.Peek()
	a.printDryRunSnapshot(metrics)
}

// printDryRunSnapshot prints the snapshot in dry-run format.
func (a *Agent) printDryRunSnapshot(metrics map[string]interface{}) {
	elapsed := time.Since(a.startTime).Round(time.Second)
	now := time.Now().UTC().Format(time.RFC3339)

	fmt.Println()
	fmt.Println("───────────────────────────────────────────────────────────")
	fmt.Printf(" SNAPSHOT @ %s (%s elapsed)\n", now, elapsed)
	fmt.Println("───────────────────────────────────────────────────────────")

	// Source stats
	for _, proc := range a.processors {
		fmt.Printf(" Source: %s\n", proc.source.Path)
		fmt.Printf("   Lines parsed:   %d\n", proc.linesParsed.Load())
		fmt.Printf("   Lines matched:  %d\n", proc.linesMatched.Load())
		fmt.Printf("   Parse errors:   %d\n", proc.parseErrors.Load())
		fmt.Println()
	}

	// Metrics table
	fmt.Println(" Aggregated Metrics:")
	fmt.Println(" ┌─────────────────────────────┬──────────┬────────────────┐")
	fmt.Println(" │ Metric                      │ Type     │ Value          │")
	fmt.Println(" ├─────────────────────────────┼──────────┼────────────────┤")

	for _, proc := range a.processors {
		for _, m := range proc.metrics {
			val := metrics[m.cfg.Name]
			valStr := formatValue(val)
			fmt.Printf(" │ %-27s │ %-8s │ %14s │\n", m.cfg.Name, m.cfg.Type, valStr)
		}
	}

	fmt.Println(" └─────────────────────────────┴──────────┴────────────────┘")
	fmt.Println()

	if a.dryRun {
		fmt.Printf(" [DRY-RUN] Would send to %s\n", a.cfg.ServerURL)
	}
	fmt.Println("───────────────────────────────────────────────────────────")
}

// formatValue formats a metric value for display.
func formatValue(v interface{}) string {
	switch val := v.(type) {
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%.2f", val)
	case int:
		return fmt.Sprintf("%d", val)
	case int64:
		return fmt.Sprintf("%d", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// stopTailers stops all tailers.
func (a *Agent) stopTailers() {
	for _, t := range a.tailers {
		if err := t.Stop(); err != nil {
			a.logger.Error("error stopping tailer", "path", t.Path(), "error", err)
		}
	}
	a.tailers = nil
}

// GetAggregator returns the aggregator (for testing).
func (a *Agent) GetAggregator() *aggregator.Aggregator {
	return a.aggregator
}

// ProcessLine processes a line for a specific source (for testing).
func (a *Agent) ProcessLine(sourceIndex int, line string) {
	if sourceIndex >= 0 && sourceIndex < len(a.processors) {
		a.processors[sourceIndex].processLine(line)
	}
}

// ProcessFile processes an entire file through the first source processor.
func (a *Agent) ProcessFile(path string) (int, error) {
	if len(a.processors) == 0 {
		return 0, fmt.Errorf("no processors configured")
	}

	proc := a.processors[0]
	return tailer.ProcessFile(path, proc.processLine, 0)
}
