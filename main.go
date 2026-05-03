package main

import (
	"context"
	"flag"
	"log"
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

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	dc, err := dockerclient.New(ctx)
	if err != nil {
		log.Fatalf("Failed to connect to Docker daemon: %v", err)
	}
	defer dc.Close()

	checker := updater.NewChecker(dc)

	log.Printf("Dockdater started (interval: %s)", *interval)

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	check(ctx, checker)

	for {
		select {
		case <-ctx.Done():
			log.Println("Dockdater stopped")
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
			log.Printf("Warning: invalid DOCKDATER_INTERVAL %q, using default: %v", v, err)
			return 5 * time.Minute
		}
		return d
	}
	return 5 * time.Minute
}

func check(ctx context.Context, checker *updater.Checker) {
	log.Println("Checking for image updates...")
	if err := checker.CheckAndUpdate(ctx); err != nil {
		log.Printf("Error during check: %v", err)
	}
}
