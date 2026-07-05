package transfers

import (
	"context"
	"log/slog"
	"time"
	"transfers/internal/domain"
	"transfers/pb"

	"github.com/xligenda/spworlds"
	"golang.org/x/sync/errgroup"
)

func ctxErr(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func (s *TransferService) selectReceiver(ctx context.Context, id string) error {
	if err := ctxErr(ctx); err != nil {
		return err
	}

	var (
		receiverID string
		transfer   *domain.Transfer
	)

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error { // receiver may be equals to sender
		selected, err := s.findReceiverWithCards(gCtx)
		if err != nil {
			return err
		}
		receiverID = selected
		return nil
	})

	g.Go(func() error {
		t, err := s.repo.FindTransfer(gCtx, id)
		if err != nil {
			return err
		}
		transfer = t
		return nil
	})

	if err := g.Wait(); err != nil {
		if transfer != nil {
			s.failTransfer(ctx, id, transfer.Sender, transfer.Amount, err,
				"failed to mark transfer as failed after receiver selection error",
			)
		}
		return mapRepoErr(err)
	}

	if transfer.Status != domain.StatusPaid {
		return ErrIncorrectState
	}

	if err := ctxErr(ctx); err != nil {
		return err
	}

	leaseUntil := time.Now().Add(selectReceiverLeaseDuration)

	leased, err := s.repo.LeaseReceiver(ctx, id, receiverID, leaseUntil)
	if err != nil {
		s.failTransfer(ctx, id, transfer.Sender, transfer.Amount, err,
			"failed to mark transfer as failed after lease error",
		)
		return ErrFailedToLeaseReceiver
	}
	if !leased {
		return ErrReceiverAlreadyLeased
	}

	return nil
}

func (s *TransferService) setStatusToNotSelected(ctx context.Context, id string) error {
	if err := ctxErr(ctx); err != nil {
		return err
	}
	return s.repo.SetStatusNotSelected(ctx, id)
}

func (s *TransferService) autoConfirmSelection(ctx context.Context, id string) error {
	if err := ctxErr(ctx); err != nil {
		return err
	}

	t, err := s.repo.FindTransfer(ctx, id)
	if err != nil {
		return err
	}

	if t.Status != domain.StatusNotSelected {
		return ErrIncorrectState
	}

	cards, err := s.spw.UserCards(ctx, *t.Receiver)
	if err != nil {
		return ErrUserCardsInvalid
	}

	if len(cards) < 1 {
		s.failTransfer(ctx, id, t.Sender, t.Amount, ErrUserCardsInvalid,
			"user has no cards and time is out",
		)
		return ErrUserCardsInvalid
	}

	comment := anonymousComment(t.Comment)
	if !t.Anonymous {
		if sender, err := s.players.GetPlayer(ctx, &pb.PlayerRequest{
			Identifier: &pb.PlayerRequest_Uuid{Uuid: t.Sender},
		}); err == nil {
			comment = identifiedComment(sender.Username, t.Comment)
		}
	}

	if err := ctxErr(ctx); err != nil {
		return err
	}

	if _, err := s.spw.CreateTransaction(ctx, spworlds.NewCreateTransactionOptions(cards[0].Number, t.Amount).SetComment(comment)); err != nil {
		s.failTransfer(ctx, id, t.Sender, t.Amount, err,
			"failed to mark transfer as failed after transaction error",
		)
		return ErrFailedToCreateTransaction
	}

	if err := s.repo.SetSent(ctx, id); err != nil {
		s.logger.ErrorContext(ctx, "transfer completed but failed to mark as sent: manual reconciliation required",
			slog.String("transfer_id", id),
			slog.Any("error", err),
		)
	}

	return nil
}

func (s *TransferService) findReleasedTransfers(
	ctx context.Context,
	limit int,
	leaseDuration time.Duration,
) ([]domain.ReleasedTransfer, error) {
	return s.repo.FindAndLeaseTransfers(ctx, limit, leaseDuration)
}
