package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	dockerclient "github.com/rdoy-lab/dockdater/internal/docker"
	"github.com/rdoy-lab/dockdater/internal/updater"
)

func main() {
	interval := flag.Duration("interval", getInterval(), "Check interval for image updates")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	dc, err := dockerclient.New(ctx)
	if err != nil {
		slog.Error("failed to connect to Docker daemon", "error", err)
		os.Exit(1)
	}
	defer dc.Close()

	checker := updater.NewChecker(dc)

	slog.Info("dockdater started", "interval", *interval)

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	check(ctx, checker)

	for {
		select {
		case <-ctx.Done():
			slog.Info("dockdater stopped")
			return
		case <-ticker.C:
			check(ctx, checker)
		}
	}
}

func getInterval() time.Duration {
	if v := os.Getenv("DOCKDATER_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			slog.Warn("invalid DOCKDATER_INTERVAL", "value", v, "error", err)
			return 5 * time.Minute
		}
		return d
	}
	return 5 * time.Minute
}

func check(ctx context.Context, checker *updater.Checker) {
	slog.Debug("checking for image updates")
	if err := checker.CheckAndUpdate(ctx); err != nil {
		slog.Error("check failed", "error", err)
	}
}
