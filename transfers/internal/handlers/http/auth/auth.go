package auth

import (
	"log/slog"

	"github.com/xligenda/spworlds"
	"github.com/xligenda/spworlds/spwmini"
)

type AuthProvider struct {
	spwmini   SPWAuth
	spworlds  *spworlds.Client
	jwtSecret []byte
	log       slog.Logger
}

func NewAuthProvider(
	spwmini SPWAuth,
	spworlds *spworlds.Client,
	secret string,
	log *slog.Logger,
) *AuthProvider {
	var logger slog.Logger
	if log == nil {
		logger = *slog.Default()
	} else {
		logger = *log
	}

	return &AuthProvider{
		spwmini:   spwmini,
		spworlds:  spworlds,
		jwtSecret: []byte(secret),
		log:       logger,
	}
}

type SPWAuth interface {
	CheckUser(user spwmini.User) bool
}
