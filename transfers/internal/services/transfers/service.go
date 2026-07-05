package transfers

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"transfers/internal/domain"
	"transfers/pb"

	"github.com/xligenda/spworlds"
	"google.golang.org/grpc"
)

type TransferRepo interface {
	CreateTransfer(ctx context.Context, t *domain.Transfer) error
	FindTransfer(ctx context.Context, id string) (*domain.Transfer, error)
	ConfirmPayment(ctx context.Context, id string) (bool, error)
	LeaseReceiver(ctx context.Context, id, receiver string, leaseUntil time.Time) (bool, error)
	SetSent(ctx context.Context, id string) error
	SetFailed(ctx context.Context, id string, reason string) error
	UserTransfers(
		ctx context.Context,
		uuid string,
		limit int,
	) ([]*domain.Transfer, error)
	SetStatusNotSelected(ctx context.Context, id string) error
	FindAndLeaseTransfers(ctx context.Context, limit int, leaseDuration time.Duration) ([]domain.ReleasedTransfer, error)
}

type PlayerServiceClient interface {
	GetRandomServerPlayer(ctx context.Context, in *pb.ServerAdress, opts ...grpc.CallOption) (*pb.ServerPlayerResponse, error)
	GetPlayer(ctx context.Context, in *pb.PlayerRequest, opts ...grpc.CallOption) (*pb.Player, error)
}

const (
	redirectURL                 = "https://discord.com/"
	webhookURL                  = "https://discord.com/"
	productName                 = "Перевод случайному игроку онлайн"
	rollbackMessage             = "Произошла ошибка и деньги были возвращены на ваш баланс"
	maxAttemptsFindReceiver     = 5
	selectReceiverLeaseDuration = 30 * time.Minute
	searchLimit                 = 30
)

var (
	anonymousComment = func(comment *string) string {
		if comment == nil || *comment == "" {
			return "Анонимный перевод!"
		}
		return fmt.Sprintf("Анонимный перевод! %s", *comment)
	}

	identifiedComment = func(username string, comment *string) string {
		if comment == nil || *comment == "" {
			return fmt.Sprintf("Перевод от %s!", username)
		}
		return fmt.Sprintf("Перевод от %s! %s", username, *comment)
	}
)

type TransferService struct {
	repo         TransferRepo
	spw          *spworlds.Client
	players      PlayerServiceClient
	logger       *slog.Logger
	mcServerAddr string
}

func NewTransferService(
	repo TransferRepo,
	spw *spworlds.Client,
	players PlayerServiceClient,
	logger *slog.Logger,
	mcServerAddr string,
) *TransferService {
	return &TransferService{
		repo:         repo,
		spw:          spw,
		players:      players,
		logger:       logger,
		mcServerAddr: mcServerAddr,
	}
}
