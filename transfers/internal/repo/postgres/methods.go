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
	row := r.db.QueryRowContext(ctx, q, id)
	t, err := scanTransfer(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("find transfer: %w", err)
	}
	return t, nil
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
		t, err := scanTransfer(rows)
		if err != nil {
			return nil, fmt.Errorf("scan user transfer: %w", err)
		}
		transfers = append(transfers, t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user transfers: %w", err)
	}

	return transfers, nil
}

func (r *TransferRepo) FindAndLeaseTransfers(ctx context.Context, limit int, leaseDuration time.Duration) ([]domain.ReleasedTransfer, error) {
	const q = `
        WITH target_transfers AS (
            SELECT id 
            FROM transfers
            WHERE status = ANY($1::text[])
              AND (leased_until IS NULL OR leased_until < NOW())
            ORDER BY created_at ASC
            LIMIT $2
            FOR UPDATE SKIP LOCKED
        )
        UPDATE transfers
        SET leased_until = NOW() + $3::interval
        FROM target_transfers
        WHERE transfers.id = target_transfers.id
        RETURNING transfers.id, transfers.status;
    `

	statuses := []string{
		domain.StatusPaid.String(),
		domain.StatusSelectedReceiver.String(),
		domain.StatusNotSelected.String(),
	}

	intervalStr := fmt.Sprintf("%d second", int(leaseDuration.Seconds()))

	rows, err := r.db.QueryContext(ctx, q, statuses, limit, intervalStr)
	if err != nil {
		return nil, fmt.Errorf("lease transfers query failed: %w", err)
	}
	defer rows.Close()

	var result []domain.ReleasedTransfer
	for rows.Next() {
		var id, status string
		if err := rows.Scan(&id, &status); err != nil {
			return nil, fmt.Errorf("scan leased transfer: %w", err)
		}
		result = append(result, domain.ReleasedTransfer{ID: id, Status: domain.Status(status)})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate leased transfers: %w", err)
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

func (r *TransferRepo) SetStatusNotSelected(ctx context.Context, id string) error {
	const q = `
        UPDATE transfers SET
            status       = $2,
            leased_until = NULL
        WHERE id = $1
          AND status = $3
          AND leased_until >= NOW()
    `
	res, err := r.db.ExecContext(ctx, q,
		id, domain.StatusNotSelected.String(), domain.StatusSelectedReceiver.String(),
	)
	if err != nil {
		return fmt.Errorf("set status not selected: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("lock expired or invalid status: %w", sql.ErrNoRows)
	}
	return nil
}

func (r *TransferRepo) LeaseReceiver(ctx context.Context, id, receiver string, leaseUntil time.Time) (bool, error) {
	const q = `
        UPDATE transfers SET
            receiver     = $2,
            status       = $3,
            leased_until = $4
        WHERE id = $1
          AND status = $5
          AND (leased_until IS NULL OR leased_until < NOW())
    `
	res, err := r.db.ExecContext(ctx, q,
		id, receiver, domain.StatusSelectedReceiver.String(), leaseUntil, domain.StatusPaid.String(),
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
          AND status IN ($3, $4)
          AND leased_until >= NOW()
    `
	res, err := r.db.ExecContext(ctx, q,
		id,
		domain.StatusSent.String(),
		domain.StatusSelectedReceiver.String(),
		domain.StatusNotSelected.String(),
	)
	if err != nil {
		return fmt.Errorf("set sent: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("lock expired or invalid status: %w", sql.ErrNoRows)
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
          AND status IN ($4, $5)
          AND leased_until >= NOW()
    `
	res, err := r.db.ExecContext(ctx, q,
		id,
		domain.StatusFailed.String(),
		reason,
		domain.StatusSelectedReceiver.String(),
		domain.StatusNotSelected.String(),
	)
	if err != nil {
		return fmt.Errorf("set failed: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("lock expired or invalid status: %w", sql.ErrNoRows)
	}
	return nil
}
