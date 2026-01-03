// SPDX-License-Identifier: MIT

// shm-agent is an autonomous agent that collects metrics from log files.
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/alecthomas/kong"
	"github.com/kolapsis/shm-agent/agent"
	"github.com/kolapsis/shm-agent/agent/config"
	"github.com/kolapsis/shm-agent/agent/tailer"
)

// CLI represents the command-line interface.
type CLI struct {
	Config   string        `short:"c" name:"config" help:"Path to configuration file" type:"existingfile" required:""`
	DryRun   bool          `name:"dry-run" help:"Print metrics without sending to server"`
	Interval time.Duration `name:"interval" help:"Override snapshot interval"`
	Verbose  int           `short:"v" name:"verbose" type:"counter" help:"Increase verbosity (-v, -vv, -vvv)"`

	Run  RunCmd  `cmd:"" default:"withargs" help:"Run the agent (default command)"`
	Test TestCmd `cmd:"" help:"Test configuration with a log file"`
}

// RunCmd runs the agent.
type RunCmd struct{}

// TestCmd tests configuration with a file.
type TestCmd struct {
	File  string `arg:"" help:"Log file to process" type:"existingfile"`
	Lines int    `short:"n" name:"lines" help:"Limit number of lines to process" default:"0"`
}

func main() {
	var cli CLI
	ctx := kong.Parse(&cli,
		kong.Name("shm-agent"),
		kong.Description("Self-Hosted Metrics agent for log parsing and metric aggregation"),
		kong.UsageOnError(),
	)

	err := ctx.Run(&cli)
	ctx.FatalIfErrorf(err)
}

// Run executes the run command.
func (r *RunCmd) Run(cli *CLI) error {
	cfg, err := config.Load(cli.Config)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Override interval if specified
	if cli.Interval > 0 {
		cfg.Interval = cli.Interval
	}

	logger := createLogger(cli.Verbose)

	ag, err := agent.New(agent.Options{
		Config:    cfg,
		Logger:    logger,
		DryRun:    cli.DryRun,
		Verbosity: cli.Verbose,
	})
	if err != nil {
		return fmt.Errorf("creating agent: %w", err)
	}

	ctx := context.Background()
	return ag.Run(ctx)
}

// Run executes the test command.
func (t *TestCmd) Run(cli *CLI) error {
	cfg, err := config.Load(cli.Config)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	logger := createLogger(cli.Verbose)

	ag, err := agent.New(agent.Options{
		Config:    cfg,
		Logger:    logger,
		DryRun:    true,
		Verbosity: cli.Verbose,
	})
	if err != nil {
		return fmt.Errorf("creating agent: %w", err)
	}

	fmt.Printf("Testing config: %s\n", cli.Config)
	fmt.Printf("Processing file: %s\n", t.File)
	if t.Lines > 0 {
		fmt.Printf("Line limit: %d\n", t.Lines)
	}
	fmt.Println()

	// Use the first source's processor
	var linesProcessed int
	var parseErrors int

	processor := func(line string) {
		ag.ProcessLine(0, line)
		linesProcessed++
	}

	count, err := tailer.ProcessFile(t.File, processor, t.Lines)
	if err != nil {
		return fmt.Errorf("processing file: %w", err)
	}

	_ = parseErrors // TODO: track parse errors

	fmt.Printf("Lines processed: %d\n", count)
	fmt.Println()

	// Print results
	metrics := ag.GetAggregator().Peek()
	printMetrics(cfg, metrics, linesProcessed)

	return nil
}

// createLogger creates a logger based on verbosity level.
func createLogger(verbosity int) *slog.Logger {
	var level slog.Level
	switch verbosity {
	case 0:
		level = slog.LevelWarn
	case 1:
		level = slog.LevelInfo
	case 2:
		level = slog.LevelDebug
	default:
		level = slog.LevelDebug - 4 // Even more verbose
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	// Use text handler for CLI
	handler := slog.NewTextHandler(os.Stderr, opts)
	return slog.New(handler)
}

// printMetrics prints metrics in a formatted table.
func printMetrics(cfg *config.Config, metrics map[string]interface{}, linesProcessed int) {
	fmt.Println("───────────────────────────────────────────────────────────")
	fmt.Println(" TEST RESULTS")
	fmt.Println("───────────────────────────────────────────────────────────")

	for _, src := range cfg.Sources {
		fmt.Printf(" Source config: %s\n", src.Path)
		fmt.Printf("   Format: %s\n", src.Format)
		if src.Pattern != "" {
			fmt.Printf("   Pattern: %s\n", src.Pattern)
		}
		fmt.Println()
	}

	fmt.Println(" Aggregated Metrics:")
	fmt.Println(" ┌─────────────────────────────┬──────────┬────────────────┐")
	fmt.Println(" │ Metric                      │ Type     │ Value          │")
	fmt.Println(" ├─────────────────────────────┼──────────┼────────────────┤")

	for _, src := range cfg.Sources {
		for _, m := range src.Metrics {
			val := metrics[m.Name]
			valStr := formatValue(val)
			fmt.Printf(" │ %-27s │ %-8s │ %14s │\n", m.Name, m.Type, valStr)
		}
	}

	fmt.Println(" └─────────────────────────────┴──────────┴────────────────┘")
	fmt.Println("───────────────────────────────────────────────────────────")
}

// formatValue formats a metric value for display.
func formatValue(v interface{}) string {
	if v == nil {
		return "0"
	}
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

// discardLogger returns a logger that discards all output.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
