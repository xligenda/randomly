package postgres

import (
	"database/sql"
	"transfers/internal/domain"
)

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTransfer(scanner rowScanner) (*domain.Transfer, error) {
	var (
		t                                domain.Transfer
		status                           string
		comment, receiver, failureReason sql.NullString
		leasedUntil                      sql.NullTime
	)

	err := scanner.Scan(
		&t.ID, &t.Amount, &comment, &t.Sender, &t.Anonymous, &receiver,
		&status, &failureReason, &t.PaymentCode, &t.CreatedAt, &leasedUntil,
	)
	if err != nil {
		return nil, err
	}

	t.Status = domain.Status(status)
	if comment.Valid {
		t.Comment = &comment.String
	}
	if receiver.Valid {
		t.Receiver = &receiver.String
	}
	if failureReason.Valid {
		t.FailureReason = &failureReason.String
	}
	if leasedUntil.Valid {
		t.LeasedUntil = &leasedUntil.Time
	}

	return &t, nil
}
