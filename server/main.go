package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ulxsth/nadesiko-reversi/server/internal/localgame"
	gameruntime "github.com/ulxsth/nadesiko-reversi/server/internal/runtime"
)

type healthResponse struct {
	Status      string `json:"status"`
	GonakoReady bool   `json:"gonakoReady"`
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
	ruleFile := filepath.Join(*rulesDir, "game", "main.nako3")
	rules, err := gameruntime.New(gameruntime.Config{
		GonakoPath: gonakoBin,
		RulesPath:  ruleFile,
		Timeout:    5 * time.Second,
	})
	if err != nil {
		log.Fatalf("ルールruntimeを初期化できません: %v", err)
	}
	game := localgame.NewService(rules)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, healthResponse{Status: "ok", GonakoReady: rules.Ready()})
	})
	mux.Handle("POST /api/local/runtime", localgame.NewHandler(game))
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
