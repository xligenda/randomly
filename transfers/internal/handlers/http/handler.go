package http

import (
	"context"
	"log/slog"
	"net/http"
	"transfers/internal/domain"

	"github.com/xligenda/spworlds"
)

type TransferService interface {
	CreateTransfer(
		ctx context.Context,
		sender string,
		amount int,
		comment *string,
		anonymous bool,
	) (*domain.Transfer, error)
	FindTransfer(ctx context.Context, id string) (*domain.Transfer, error)
	ConfirmPayment(
		ctx context.Context,
		payment *spworlds.PaymentData,
	) (*domain.Transfer, error)
	ConfirmSelection(
		ctx context.Context,
		id, receiver, card string,
	) (*domain.Transfer, error)
	UserTransfers(
		ctx context.Context,
		uuid string,
	) ([]*domain.Transfer, error)
}

type AuthProvider interface {
	AuthInit(w http.ResponseWriter, r *http.Request)
	JWTMiddleware(next http.HandlerFunc) http.HandlerFunc
	ValidateWebhookMiddleware(next http.HandlerFunc) http.HandlerFunc
}

type Handler struct {
	auth     AuthProvider
	spworlds *spworlds.Client
	svc      TransferService
	log      *slog.Logger
}

func NewHandler(
	auth AuthProvider,
	spworlds *spworlds.Client,
	svc TransferService,
	log *slog.Logger,
) *Handler {
	if log == nil {
		log = slog.Default()
	}
	return &Handler{
		auth:     auth,
		spworlds: spworlds,
		svc:      svc,
		log:      log,
	}
}

func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("POST /auth/init", h.auth.AuthInit)

	mux.HandleFunc("POST /webhooks/spworlds/payment", h.auth.ValidateWebhookMiddleware(h.paymentWebhook))

	mux.HandleFunc("POST /transfer", h.auth.JWTMiddleware(h.createTransfer))
	mux.HandleFunc("GET /transfer/{id}", h.auth.JWTMiddleware(
		h.ensureOwner(h.getTransfer),
	))
	mux.HandleFunc("POST /transfer/{id}/select", h.auth.JWTMiddleware(
		h.ensureOwner(h.confirmSelection),
	))
	mux.HandleFunc("GET /transfers/@me", h.auth.JWTMiddleware(h.getMyTransfers))
}
