package transfers

import (
	"context"
	"log/slog"
	"transfers/internal/domain"
	"transfers/pb"

	"github.com/google/uuid"
	"github.com/xligenda/spworlds"
	"golang.org/x/sync/errgroup"
)

func (s *TransferService) CreateTransfer(
	ctx context.Context,
	sender string,
	amount int,
	comment *string,
	anonymous bool,
) (*domain.Transfer, error) {
	id := uuid.New().String()

	products, err := parseAmountToProducts(productName, amount)
	if err != nil {
		return nil, err
	}

	payment, err := s.spw.CreatePayment(ctx,
		spworlds.NewCreatePaymentOptions(products, redirectURL, webhookURL).
			SetPayload(paymentPayload(id, amount)),
	)
	if err != nil {
		return nil, ErrFailedToCreatePayment
	}

	t := domain.NewTransfer(id, sender, amount, comment, anonymous, payment.Code)

	if err := s.repo.CreateTransfer(ctx, &t); err != nil {
		s.logger.ErrorContext(ctx, "failed to persist transfer after payment creation",
			slog.String("transfer_id", id),
			slog.Any("error", err),
		)
		return nil, ErrFailedToPersist
	}

	return &t, nil
}

func (s *TransferService) FindTransfer(ctx context.Context, id string) (*domain.Transfer, error) {
	t, err := s.repo.FindTransfer(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	return t, nil
}

func (s *TransferService) ConfirmPayment(
	ctx context.Context,
	payment *spworlds.PaymentData,
) (*domain.Transfer, error) {
	payload, err := parsePaymentPayload(payment.Payload)
	if err != nil {
		return nil, ErrFailedToParse
	}

	t, err := s.repo.FindTransfer(ctx, payload.ID)
	if err != nil {
		s.rollbackPayment(ctx, payload.ID, payment.Payer, payload.Amount, err)
		return nil, mapRepoErr(err)
	}

	if t.Status != domain.StatusCreated {
		s.rollbackPayment(ctx, payload.ID, payment.Payer, payload.Amount, ErrIncorrectState)
		return nil, ErrIncorrectState
	}

	confirmed, err := s.repo.ConfirmPayment(ctx, t.ID)
	if err != nil {
		s.rollbackPayment(ctx, t.ID, t.Sender, t.Amount, err)
		return nil, ErrFailedToConfirmPayment
	}

	if !confirmed {
		s.logger.InfoContext(ctx, "duplicate payment webhook ignored",
			slog.String("transfer_id", t.ID),
		)
		return t, nil
	}

	t.ConfirmPayment()
	return t, nil
}

func (s *TransferService) ConfirmSelection(
	ctx context.Context,
	id, receiver, card string,
) (*domain.Transfer, error) {
	var (
		cards []spworlds.Card
		t     *domain.Transfer
	)

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		c, err := s.spw.UserCards(gCtx, receiver)
		if err != nil {
			return ErrUserCardsInvalid
		}
		cards = c
		return nil
	})

	g.Go(func() error {
		tr, err := s.repo.FindTransfer(gCtx, id)
		if err != nil {
			return err
		}
		t = tr
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, mapRepoErr(err)
	}

	if t.Status != domain.StatusSelectedReceiver {
		return nil, ErrIncorrectState
	}

	if len(cards) == 0 {
		s.failTransfer(ctx, id, t.Sender, t.Amount, ErrUserCardsInvalid,
			"failed to mark transfer as failed after empty cards",
		)
		return nil, ErrUserCardsInvalid
	}

	if !cardBelongsTo(cards, card) {
		return nil, ErrSelectionCardMismatch
	}

	player, err := s.players.GetPlayer(ctx, &pb.PlayerRequest{
		Identifier: &pb.PlayerRequest_Uuid{Uuid: *t.Receiver},
	})
	if err != nil {
		return nil, ErrFailedToFetchMCProfile
	}
	if player.Username != receiver {
		return nil, ErrSelectionReceiverMismatch
	}

	comment := anonymousComment(t.Comment)
	if !t.Anonymous {
		if sender, err := s.players.GetPlayer(ctx, &pb.PlayerRequest{
			Identifier: &pb.PlayerRequest_Uuid{Uuid: t.Sender},
		}); err == nil {
			comment = identifiedComment(sender.Username, t.Comment)
		}
	}

	if _, err := s.spw.CreateTransaction(ctx, spworlds.NewCreateTransactionOptions(card, t.Amount).SetComment(comment)); err != nil {
		s.failTransfer(ctx, id, t.Sender, t.Amount, err,
			"failed to mark transfer as failed after transaction error",
		)
		return nil, ErrFailedToCreateTransaction
	}

	if err := s.repo.SetSent(ctx, id); err != nil {
		s.logger.ErrorContext(ctx, "transfer completed but failed to mark as sent — manual reconciliation required",
			slog.String("transfer_id", id),
			slog.Any("error", err),
		)
	}

	t.SetPaid()
	return t, nil
}

func (s *TransferService) UserTransfers(
	ctx context.Context,
	uuid string,
) ([]*domain.Transfer, error) {
	return s.repo.UserTransfers(ctx, uuid, searchLimit)
}
