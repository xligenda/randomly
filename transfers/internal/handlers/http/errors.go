package http

import (
	"errors"
	"log/slog"
	"net/http"
	"transfers/internal/services/transfers"
)

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{
		Code:    http.StatusText(status),
		Message: msg,
	})
}

func (h *Handler) handleServiceError(w http.ResponseWriter, op string, err error, args ...any) {
	if errors.Is(err, transfers.ErrNotFound) {
		writeError(w, http.StatusNotFound, "transfer not found")
		return
	}

	args = append(args, slog.String("err", err.Error()))
	h.log.Error(op+": service error", args...)
	writeError(w, http.StatusInternalServerError, "failed to "+op)
}
