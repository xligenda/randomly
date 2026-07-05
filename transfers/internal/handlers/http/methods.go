package http

import (
	"log/slog"
	"net/http"
	"transfers/internal/handlers/http/auth"
	"transfers/pb"
)

type createTransferRequest struct {
	Sender    string  `json:"sender"`
	Amount    int     `json:"amount"`
	Comment   *string `json:"comment,omitempty"`
	Anonymous bool    `json:"anonymous"`
}

// POST /transfer
func (h *Handler) createTransfer(w http.ResponseWriter, r *http.Request) {
	var req createTransferRequest
	if !h.decodeBody(w, r, "createTransfer", &req) {
		return
	}
	if requireField(w, req.Sender, "sender is required") {
		return
	}
	if req.Amount <= 0 {
		writeError(w, http.StatusBadRequest, "amount must be positive")
		return
	}

	transfer, err := h.svc.CreateTransfer(r.Context(), req.Sender, req.Amount, req.Comment, req.Anonymous)
	if err != nil {
		h.handleServiceError(w, "create transfer", err)
		return
	}

	h.log.Info("transfer created", slog.String("id", transfer.ID))
	writeJSON(w, http.StatusCreated, toPublic(*transfer))
}

// GET /transfer/{id}
func (h *Handler) getTransfer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if requireField(w, id, "id is required") {
		return
	}

	transfer, err := h.svc.FindTransfer(r.Context(), id)
	if err != nil {
		h.handleServiceError(w, "get transfer", err, slog.String("id", id))
		return
	}

	writeJSON(w, http.StatusOK, toPublic(*transfer))
}

type confirmSelectionRequest struct {
	Receiver string `json:"receiver"`
	Card     string `json:"card"`
}

// POST /transfer/{id}/select
func (h *Handler) confirmSelection(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if requireField(w, id, "id is required") {
		return
	}

	var req confirmSelectionRequest
	if !h.decodeBody(w, r, "confirmSelection", &req) {
		return
	}
	if requireField(w, req.Receiver, "receiver is required") {
		return
	}
	if requireField(w, req.Card, "card is required") {
		return
	}

	transfer, err := h.svc.ConfirmSelection(r.Context(), id, req.Receiver, req.Card)
	if err != nil {
		h.handleServiceError(w, "confirm selection", err, slog.String("id", id))
		return
	}

	writeJSON(w, http.StatusOK, toPublic(*transfer))
}

// POST /webhooks/spworlds/payment
func (h *Handler) paymentWebhook(w http.ResponseWriter, r *http.Request) {
	payment, err := h.spworlds.ParsePaymentData(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	transfer, err := h.svc.ConfirmPayment(r.Context(), payment)
	if err != nil {
		h.handleServiceError(w, "process payment", err)
		return
	}

	h.log.Info("payment confirmed", slog.String("transfer_id", transfer.ID))
	writeJSON(w, http.StatusOK, toPublic(*transfer))
}

// GET /transfers/@me
func (h *Handler) getMyTransfers(w http.ResponseWriter, r *http.Request) {
	userUUID, ok := r.Context().Value(auth.UserUUIDKey).(string)
	if !ok || userUUID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	transfers, err := h.svc.UserTransfers(r.Context(), userUUID)
	if err != nil {
		h.handleServiceError(w, "get user transfers", err, slog.String("user_uuid", userUUID))
		return
	}

	publicTransfers := make([]PublicTransfer, len(transfers))
	for i, t := range transfers {
		publicTransfers[i] = toPublic(*t)
	}

	writeJSON(w, http.StatusOK, publicTransfers)
}

type serverData struct {
	Online int `json:"online"`
}

func (h *Handler) fetchServerData(w http.ResponseWriter, r *http.Request) {
	resp, err := h.players.ServerOnline(r.Context(), &pb.ServerAdress{
		Address: h.mcServerAddr,
	})

	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, serverData{
		Online: int(resp.GetOnlineCount()),
	})
}

// GET /health
func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
