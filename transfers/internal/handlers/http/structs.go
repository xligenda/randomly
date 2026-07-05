package http

import (
	"transfers/internal/domain"
)

type PublicTransfer struct {
	ID            string  `json:"id"`
	Amount        int     `json:"amount"`
	Comment       *string `json:"comment,omitempty"`
	Receiver      *string `json:"receiver,omitempty"`
	Status        string  `json:"status"`
	FailureReason *string `json:"failure_reason,omitempty"`
	PaymentCode   string  `json:"payment_code,omitempty"`
	CreatedAt     int64   `json:"created_at"` // unix timestamp
}

func toPublic(t domain.Transfer) PublicTransfer {
	return PublicTransfer{
		ID:            t.ID,
		Amount:        t.Amount,
		Comment:       t.Comment,
		Status:        t.Status.String(),
		Receiver:      t.Receiver,
		FailureReason: t.FailureReason,
		PaymentCode:   t.PaymentCode,
		CreatedAt:     t.CreatedAt.Unix(),
	}
}
