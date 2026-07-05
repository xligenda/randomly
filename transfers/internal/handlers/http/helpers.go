package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func decodeJSON(r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func requireField(w http.ResponseWriter, value string, message string) bool {
	if value == "" {
		writeError(w, http.StatusBadRequest, message)
		return true
	}
	return false
}

func (h *Handler) decodeBody(w http.ResponseWriter, r *http.Request, op string, dst any) bool {
	if err := decodeJSON(r, dst); err != nil {
		h.log.Warn(op+": decode error", slog.String("err", err.Error()))
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}
