package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// 에러 응답 형식은 { "error": "..." } 하나로 통일한다.
type errorBody struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Warn("write response", "err", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorBody{Error: msg})
}

// internalError 는 원인을 로그에만 남긴다. 응답에 DB/ES 에러 원문을 싣지 않는다.
func internalError(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("request failed", "path", r.URL.Path, "err", err)
	writeError(w, http.StatusBadGateway, "데이터 소스 조회에 실패했다")
}
