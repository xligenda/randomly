package domain

import (
	"time"
)

type Transfer struct {
	ID            string     `json:"id"`
	Amount        int        `json:"amount"`
	Comment       *string    `json:"comment,omitempty"`
	Sender        string     `json:"sender"`
	Anonymous     bool       `json:"anonymous"`
	Receiver      *string    `json:"receiver,omitempty"`
	Status        Status     `json:"status"`
	FailureReason *string    `json:"failure_reason,omitempty"`
	PaymentCode   string     `json:"payment_code,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	LeasedUntil   *time.Time `json:"-"`
}

type ReleasedTransfer struct {
	ID     string
	Status Status
}

func NewTransfer(
	id string,
	sender string,
	amount int,
	comment *string,
	anonymous bool,
	paymentCode string,
) Transfer {
	return Transfer{
		ID:          id,
		Sender:      sender,
		Amount:      amount,
		Comment:     comment,
		Anonymous:   anonymous,
		Status:      StatusCreated,
		PaymentCode: paymentCode,
		CreatedAt:   time.Now(),
	}
}

func (t *Transfer) ConfirmPayment() {
	t.Status = StatusPaid
	t.Release()
}

func (t *Transfer) SelectReceiver(receiver string) {
	t.Status = StatusSelectingCard
	// should add lease, so don't release
}

func (t *Transfer) SetFailrule(err error) {
	t.FailureReason = new(err.Error())
	t.Status = StatusFailed
	t.Release()
}

func (t *Transfer) SetPaid() {
	t.Status = StatusPaid
}

func (t *Transfer) Release() {
	t.LeasedUntil = nil
}

func (t *Transfer) SetLease(u time.Time) {
	t.LeasedUntil = &u
}
