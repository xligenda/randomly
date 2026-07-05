package auth

import (
	"net/http"
)

func (p *AuthProvider) ValidateWebhookMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pass, err := p.spworlds.ValidateRequest(r)
		if err != nil || !pass {
			writeError(w, http.StatusUnauthorized, "unauthorized: error validating webhook")
			return
		}

		next(w, r)
	}
}
