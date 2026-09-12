package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type healthResponse struct {
	Status      string `json:"status"`
	GonakoReady bool   `json:"gonakoReady"`
}

type smokeResponse struct {
	OK     bool   `json:"ok"`
	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

func main() {
	addr := flag.String("addr", "127.0.0.1:4173", "listen address")
	webDir := flag.String("web", "web", "web root")
	rulesDir := flag.String("rules", "rules", "rule source directory")
	flag.Parse()

	gonakoBin := os.Getenv("GONAKO_BIN")
	if gonakoBin == "" {
		gonakoBin = filepath.Join(".tools", "bin", "gonako")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, err := os.Stat(gonakoBin)
		writeJSON(w, http.StatusOK, healthResponse{Status: "ok", GonakoReady: err == nil})
	})
	mux.HandleFunc("GET /api/runtime/smoke", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, gonakoBin, filepath.Join(*rulesDir, "smoke.nako3"))
		output, err := cmd.CombinedOutput()
		if ctx.Err() == context.DeadlineExceeded {
			writeJSON(w, http.StatusGatewayTimeout, smokeResponse{Error: "gonako timed out"})
			return
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, smokeResponse{Error: err.Error(), Output: strings.TrimSpace(string(output))})
			return
		}
		writeJSON(w, http.StatusOK, smokeResponse{OK: true, Output: strings.TrimSpace(string(output))})
	})
	mux.Handle("/", http.FileServer(http.Dir(*webDir)))

	server := &http.Server{
		Addr:              *addr,
		Handler:           withHeaders(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("なでしこ・リバーシ開発サーバー: http://%s", *addr)
	log.Fatal(server.ListenAndServe())
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("JSON応答エラー: %v", err)
	}
}

func withHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

