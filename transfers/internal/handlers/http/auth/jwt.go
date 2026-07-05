package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"log/slog"

	"github.com/golang-jwt/jwt/v5"
	"github.com/xligenda/spworlds/spwmini"
)

type TokenClaims struct {
	jwt.RegisteredClaims
	UUID string `json:"uuid"`
}

type contextKey string

const UserUUIDKey contextKey = "user_uuid"

func (p *AuthProvider) AuthInit(w http.ResponseWriter, r *http.Request) {
	var initData spwmini.User
	if err := json.NewDecoder(r.Body).Decode(&initData); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if valid := p.spwmini.CheckUser(initData); !valid {
		writeError(w, http.StatusUnauthorized, "invalid spworlds signature")
		return
	}

	userID := initData.MinecraftUUID
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "invalid user data in sdk object")
		return
	}

	token, err := generateToken(p.jwtSecret, userID, 4*time.Hour)
	if err != nil {
		p.handleServiceError(w, "generate token", err)
		return
	}

	p.log.Info("auth token generated", slog.String("uuid", userID))
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (p *AuthProvider) JWTMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeError(w, http.StatusUnauthorized, "authorization header missing")
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			writeError(w, http.StatusUnauthorized, "invalid authorization format")
			return
		}

		claims, err := parseAndValidateToken(p.jwtSecret, parts[1])
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized: "+err.Error())
			return
		}

		ctx := context.WithValue(r.Context(), UserUUIDKey, claims.UUID)
		next(w, r.WithContext(ctx))
	}
}
