package transfers

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"transfers/internal/domain"
)

type WorkerConfig struct {
	PollInterval  time.Duration
	BatchLimit    int
	LeaseDuration time.Duration
	Concurrency   int
}

type EventListener interface {
	Listen(ctx context.Context, channel string, wakeUp chan<- struct{}) error
}

type Worker struct {
	svc      *TransferService
	logger   *slog.Logger
	cfg      WorkerConfig
	listener EventListener
	wakeUp   chan struct{}
}

func NewWorker(svc *TransferService, logger *slog.Logger, cfg WorkerConfig, listener EventListener) *Worker {
	return &Worker{
		svc:      svc,
		logger:   logger,
		cfg:      cfg,
		listener: listener,
		wakeUp:   make(chan struct{}, 1),
	}
}

func (w *Worker) Run(ctx context.Context) error {
	go func() {
		w.logger.InfoContext(ctx, "starting database event listener on channel 'transfer_event'...")
		if err := w.listener.Listen(ctx, "transfer_event", w.wakeUp); err != nil {
			if !errors.Is(err, context.Canceled) {
				w.logger.ErrorContext(ctx, "event listener critical error", slog.Any("error", err))
			}
		}
	}()

	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	w.triggerTick(ctx)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-w.wakeUp:
			w.triggerTick(ctx)

		case <-ticker.C:
			w.triggerTick(ctx)
		}
	}
}

func (w *Worker) triggerTick(ctx context.Context) {
	found, err := w.tick(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return
		}
		w.logger.ErrorContext(ctx, "worker tick failed", slog.Any("error", err))
		return
	}

	if found {
		select {
		case w.wakeUp <- struct{}{}:
		default:
		}
	}
}

func (w *Worker) tick(ctx context.Context) (found bool, err error) {
	transfers, err := w.svc.findReleasedTransfers(ctx, w.cfg.BatchLimit, w.cfg.LeaseDuration)
	if err != nil {
		return false, err
	}
	if len(transfers) == 0 {
		return false, nil
	}

	w.processBatch(ctx, transfers)
	return true, nil
}

func (w *Worker) processBatch(ctx context.Context, transfers []domain.ReleasedTransfer) {
	sem := make(chan struct{}, w.cfg.Concurrency)
	var wg sync.WaitGroup

	for _, t := range transfers {
		sem <- struct{}{}
		wg.Add(1)

		go func(transfer domain.ReleasedTransfer) {
			defer func() {
				<-sem
				wg.Done()
			}()
			w.processOne(ctx, transfer)
		}(t)
	}
	wg.Wait()
}

func (w *Worker) processOne(ctx context.Context, t domain.ReleasedTransfer) {
	var err error

	switch t.Status {
	case domain.StatusPaid:
		err = w.svc.selectReceiver(ctx, t.ID)
	case domain.StatusSelectedReceiver:
		err = w.svc.setStatusToNotSelected(ctx, t.ID)
	case domain.StatusNotSelected:
		err = w.svc.autoConfirmSelection(ctx, t.ID)
	default:
		w.logger.WarnContext(ctx, "unexpected released transfer status, skipping",
			slog.String("transfer_id", t.ID),
			slog.Any("status", t.Status),
		)
		return
	}

	if err != nil {
		w.logger.ErrorContext(ctx, "failed to process released transfer",
			slog.String("transfer_id", t.ID),
			slog.Any("status", t.Status),
			slog.Any("error", err),
		)
	}
}
