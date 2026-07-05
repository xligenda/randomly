package http

import (
	"log/slog"
	"net/http"
	"transfers/internal/handlers/http/auth"
)

func (h *Handler) ensureOwner(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userUUID, ok := r.Context().Value(auth.UserUUIDKey).(string)
		if !ok || userUUID == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		id := r.PathValue("id")
		if requireField(w, id, "id is required") {
			return
		}

		t, err := h.svc.FindTransfer(r.Context(), id)
		if err != nil {
			h.handleServiceError(w, "ensure owner", err, slog.String("id", id))
			return
		}

		if t.Sender != userUUID {
			writeError(w, http.StatusForbidden, "unauthorized")
			return
		}

		next(w, r)
	}
}
