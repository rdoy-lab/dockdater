package webui

import (
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/rdoy-lab/dockdater/internal/state"
)

//go:embed index.html favicon.svg
var htmlContent embed.FS

func Start(addr string, store *state.Store) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/favicon.ico", serveFavicon)
	mux.HandleFunc("/favicon.svg", serveFavicon)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		data, err := htmlContent.ReadFile("index.html")
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})

	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		containers := store.Containers()
		lastChecked := store.LastChecked()
		resp := struct {
			Containers  any    `json:"containers"`
			LastChecked string `json:"lastChecked"`
		}{containers, lastChecked}
		writeJSON(w, resp)
	})

	mux.HandleFunc("/api/deployments", func(w http.ResponseWriter, r *http.Request) {
		deps := store.Deployments()
		if deps == nil {
			deps = []state.DeploymentRow{}
		}
		writeJSON(w, deps)
	})

	server := &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("webui listen: %w", err)
	}

	slog.Info("web UI started", "addr", ln.Addr().String())

	go func() {
		if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("web UI server error", "error", err)
		}
	}()

	return nil
}

func serveFavicon(w http.ResponseWriter, r *http.Request) {
	data, err := htmlContent.ReadFile("favicon.svg")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Write(data)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}
