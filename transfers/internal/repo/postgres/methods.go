package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
	"transfers/internal/domain"
)

func (r *TransferRepo) CreateTransfer(ctx context.Context, t *domain.Transfer) error {
	var comment sql.NullString
	if t.Comment != nil {
		comment = sql.NullString{String: *t.Comment, Valid: true}
	}

	const q = `
		INSERT INTO transfers (
			id, amount, comment, sender, anonymous,
			status, payment_code, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`
	_, err := r.db.ExecContext(ctx, q,
		t.ID, t.Amount, comment, t.Sender, t.Anonymous,
		t.Status.String(), t.PaymentCode, t.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert transfer: %w", err)
	}
	return nil
}

func (r *TransferRepo) FindTransfer(ctx context.Context, id string) (*domain.Transfer, error) {
	const q = `
		SELECT id, amount, comment, sender, anonymous, receiver,
			status, failure_reason, payment_code, created_at, leased_until
		FROM transfers
		WHERE id = $1
	`
	var (
		t                                domain.Transfer
		status                           string
		comment, receiver, failureReason sql.NullString
		leasedUntil                      sql.NullTime
	)
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&t.ID, &t.Amount, &comment, &t.Sender, &t.Anonymous, &receiver,
		&status, &failureReason, &t.PaymentCode, &t.CreatedAt, &leasedUntil,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("find transfer: %w", err)
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

func (r *TransferRepo) UserTransfers(
	ctx context.Context,
	uuid string,
	limit int,
) ([]*domain.Transfer, error) {
	const q = `
        SELECT id, amount, comment, sender, anonymous, receiver,
            status, failure_reason, payment_code, created_at, leased_until
        FROM transfers
        WHERE sender = $1
        ORDER BY created_at DESC
        LIMIT $2
    `

	rows, err := r.db.QueryContext(ctx, q, uuid, limit)
	if err != nil {
		return nil, fmt.Errorf("user transfers: %w", err)
	}
	defer rows.Close()

	var transfers []*domain.Transfer
	for rows.Next() {
		var (
			t                                domain.Transfer
			status                           string
			comment, receiver, failureReason sql.NullString
			leasedUntil                      sql.NullTime
		)

		err := rows.Scan(
			&t.ID, &t.Amount, &comment, &t.Sender, &t.Anonymous, &receiver,
			&status, &failureReason, &t.PaymentCode, &t.CreatedAt, &leasedUntil,
		)
		if err != nil {
			return nil, fmt.Errorf("scan user transfer: %w", err)
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

		transfers = append(transfers, &t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user transfers: %w", err)
	}

	return transfers, nil
}
func (r *TransferRepo) FindReleasedTransfers(ctx context.Context) ([]domain.ReleasedTransfer, error) {
	const q = `
		SELECT id, status
		FROM transfers
		WHERE status IN ($1, $2)
		  AND (leased_until IS NULL OR leased_until < now())
	`
	rows, err := r.db.QueryContext(ctx, q,
		domain.StatusPaid.String(), domain.StatusSelectingCard.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("find released transfers: %w", err)
	}
	defer rows.Close()

	var result []domain.ReleasedTransfer
	for rows.Next() {
		var id, status string
		if err := rows.Scan(&id, &status); err != nil {
			return nil, fmt.Errorf("scan released transfer: %w", err)
		}
		result = append(result, domain.ReleasedTransfer{ID: id, Status: domain.Status(status)})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate released transfers: %w", err)
	}
	return result, nil
}

func (r *TransferRepo) ConfirmPayment(ctx context.Context, id string) (bool, error) {
	const q = `
		UPDATE transfers SET
			status       = $2,
			leased_until = NULL
		WHERE id = $1
		  AND status = $3
	`
	res, err := r.db.ExecContext(ctx, q,
		id, domain.StatusPaid.String(), domain.StatusCreated.String(),
	)
	if err != nil {
		return false, fmt.Errorf("confirm payment: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("rows affected: %w", err)
	}
	return n > 0, nil
}

func (r *TransferRepo) LeaseReceiver(ctx context.Context, id, receiver string, leaseUntil time.Time) (bool, error) {
	const q = `
		UPDATE transfers SET
			receiver     = $2,
			status       = $3,
			leased_until = $4
		WHERE id = $1
		  AND status = $5
		  AND (leased_until IS NULL OR leased_until < now())
	`
	res, err := r.db.ExecContext(ctx, q,
		id, receiver, domain.StatusSelectingCard.String(), leaseUntil, domain.StatusPaid.String(),
	)
	if err != nil {
		return false, fmt.Errorf("lease receiver: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("rows affected: %w", err)
	}
	return n > 0, nil
}

func (r *TransferRepo) SetSent(ctx context.Context, id string) error {
	const q = `
		UPDATE transfers SET
			status       = $2,
			leased_until = NULL
		WHERE id = $1
	`
	res, err := r.db.ExecContext(ctx, q, id, domain.StatusSent.String())
	if err != nil {
		return fmt.Errorf("set sent: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *TransferRepo) SetFailed(ctx context.Context, id string, reason string) error {
	const q = `
		UPDATE transfers SET
			status         = $2,
			failure_reason = $3,
			leased_until   = NULL
		WHERE id = $1
	`
	res, err := r.db.ExecContext(ctx, q, id, domain.StatusFailed.String(), reason)
	if err != nil {
		return fmt.Errorf("set failed: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
