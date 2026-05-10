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
	"github.com/rdoy-lab/dockdater/internal/state"
	"github.com/rdoy-lab/dockdater/internal/updater"
	"github.com/rdoy-lab/dockdater/internal/webui"
)

func main() {
	interval := flag.Duration("interval", getInterval(), "Check interval for image updates")
	webAddr := flag.String("web", getWebAddr(), "Web UI listen address (e.g. :8080)")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	dc, err := dockerclient.New(ctx)
	if err != nil {
		slog.Error("failed to connect to Docker daemon", "error", err)
		os.Exit(1)
	}
	defer dc.Close()

	store := state.New()
	checker := updater.NewChecker(dc, store)

	if err := webui.Start(*webAddr, store); err != nil {
		slog.Error("failed to start web UI", "error", err)
		os.Exit(1)
	}

	slog.Info("dockdater started", "interval", *interval, "web", *webAddr)

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

func getWebAddr() string {
	if v := os.Getenv("DOCKDATER_WEB"); v != "" {
		return v
	}
	return ":8080"
}

func check(ctx context.Context, checker *updater.Checker) {
	slog.Debug("checking for image updates")
	if err := checker.CheckAndUpdate(ctx); err != nil {
		slog.Error("check failed", "error", err)
	}
}
