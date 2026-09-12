package localgame

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/ulxsth/nadesiko-reversi/server/internal/protocol"
	"github.com/ulxsth/nadesiko-reversi/server/internal/runtime"
)

const maxRequestBytes = 1 << 20

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var request protocol.Request
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, rejected(protocol.CodeInvalidJSON, "JSONを解析できません", nil))
		return
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, rejected(protocol.CodeInvalidJSON, "JSON objectは1件だけ送信してください", nil))
		return
	}

	response, err := h.service.Evaluate(r.Context(), request)
	if err != nil {
		status := http.StatusBadGateway
		message := "ルール評価に失敗しました"
		var timeoutError *runtime.TimeoutError
		if errors.As(err, &timeoutError) {
			status = http.StatusGatewayTimeout
			message = "ルール評価が時間内に完了しませんでした"
		}
		log.Printf("local runtime error: %v", err)
		writeJSON(w, status, rejected(protocol.CodeInvalidJSON, message, h.service.Snapshot()))
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("localgame JSON response error: %v", err)
	}
}
