package transfers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"transfers/pb"

	"github.com/xligenda/spworlds"
)

const (
	maxPricePerProduct = 1728
	maxCountPerProduct = 9999
	maxTotalAmount     = 25_000 // must stay well below maxPricePerProduct * maxCountPerProduct
)

func parseAmountToProducts(productName string, amount int) ([]spworlds.Product, error) {
	if amount <= 0 {
		return nil, ErrNegativeAmount
	}
	if amount > maxTotalAmount {
		return nil, ErrAmountLimitExceeded
	}

	// fits in a single product
	if amount <= maxPricePerProduct {
		return []spworlds.Product{{Name: productName, Count: 1, Price: amount}}, nil
	}

	if p, ok := singleProductFactorization(amount); ok {
		return []spworlds.Product{{Name: productName, Count: amount / p, Price: p}}, nil
	}

	return splitIntoTwoProducts(productName, amount)
}

// price <= maxPricePerProduct and count <= maxCountPerProduct
// returns the chosen price and true if found
func singleProductFactorization(amount int) (int, bool) {
	// any n below this bound can not give a count within the limit, skip them
	minN := max(
		(amount+maxCountPerProduct-1)/maxCountPerProduct,
		1,
	)

	for n := maxPricePerProduct; n >= minN; n-- {
		if amount%n == 0 {
			return n, true
		}
	}
	return 0, false
}

func splitIntoTwoProducts(productName string, amount int) ([]spworlds.Product, error) {
	const n = maxPricePerProduct
	count := amount / n
	rem := amount % n

	if count > maxCountPerProduct {
		return nil, ErrAmountLimitExceeded
	}

	products := []spworlds.Product{{Name: productName, Count: count, Price: n}}
	if rem != 0 {
		products = append(products, spworlds.Product{Name: productName, Count: 1, Price: rem})
	}
	return products, nil
}

// uuid len is 36 ch + len of max int 64 is 19 + ":" = 56 characters
func paymentPayload(uuid string, amount int) string {
	if amount < 0 {
		amount = 0
	}

	return fmt.Sprintf("%s:%d", uuid, amount)
}

type PaymentPayload struct {
	ID     string
	Amount int
}

func parsePaymentPayload(payload string) (PaymentPayload, error) {
	parts := strings.Split(payload, ":")
	if len(parts) != 2 {
		return PaymentPayload{}, ErrInvalidSignature
	}

	amount, err := strconv.Atoi(parts[1])
	if err != nil {
		return PaymentPayload{}, ErrInvalidSignature
	}

	return PaymentPayload{
		ID:     parts[0],
		Amount: amount,
	}, nil
}

func (s *TransferService) failTransfer(
	ctx context.Context,
	id, sender string,
	amount int,
	err error,
	logMsg string,
) {
	s.rollbackPayment(ctx, id, sender, amount, err)

	if repoErr := s.repo.SetFailed(ctx, id, err.Error()); repoErr != nil {
		s.logger.ErrorContext(ctx, logMsg,
			slog.String("transfer_id", id),
			slog.Any("error", repoErr),
		)
	}
}

func (s *TransferService) rollbackPayment(
	ctx context.Context,
	id, sender string,
	amount int,
	reason error,
) {
	s.logger.WarnContext(ctx, "rolling back payment",
		slog.String("transfer_id", id),
		slog.String("sender", sender),
		slog.Int("amount", amount),
		slog.Any("reason", reason),
	)

	senderCards, err := s.spw.UserCards(ctx, sender)
	if err != nil || len(senderCards) == 0 {
		s.logger.ErrorContext(ctx, "rollback failed: could not fetch sender cards",
			slog.String("transfer_id", id),
			slog.Any("error", err),
		)
		return
	}

	if _, err := s.spw.CreateTransaction(ctx,
		spworlds.NewCreateTransactionOptions(senderCards[0].Number, amount).
			SetComment(rollbackMessage),
	); err != nil {
		s.logger.ErrorContext(ctx, "rollback transaction failed — manual intervention required",
			slog.String("transfer_id", id),
			slog.Any("error", err),
		)
	}
}

func (s *TransferService) findReceiverWithCards(ctx context.Context) (string, error) {
	for range maxAttemptsFindReceiver {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		player, err := s.players.GetRandomServerPlayer(ctx, &pb.RandomServerPlayerRequest{
			Address: s.mcServerAddr,
		})
		if err != nil {
			s.logger.WarnContext(ctx, "failed to get random server player", slog.Any("error", err))
			continue
		}

		cards, err := s.spw.UserCards(ctx, player.GetUsername())
		if err != nil || len(cards) == 0 {
			continue
		}

		return player.GetId(), nil
	}

	return "", ErrNoEligibleReceiver
}

func cardBelongsTo(cards []spworlds.Card, number string) bool {
	for _, c := range cards {
		if c.Number == number {
			return true
		}
	}
	return false
}

func mapRepoErr(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
