package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpHandlers "transfers/internal/handlers/http"
	"transfers/internal/handlers/http/auth"
	"transfers/internal/repo/postgres"
	"transfers/internal/services/transfers"
	"transfers/pb"

	_ "github.com/lib/pq"
	"github.com/xligenda/spworlds"
	"github.com/xligenda/spworlds/spwmini"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type SPWmini struct {
	token string
}

func (c *SPWmini) CheckUser(user spwmini.User) bool {
	return spwmini.CheckUser(user, c.token)
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	l := slog.Default()

	dbURL := getEnv("DATABASE_URL", "")
	if dbURL == "" {
		dbURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
			getEnv("DB_USER", "postgres"),
			getEnv("DB_PASSWORD", "postgres"),
			getEnv("DB_HOST", "db"),
			getEnv("DB_PORT", "5432"),
			getEnv("DB_NAME", "postgres"),
		)
	}

	spwID := getEnv("SPW_CLIENT_ID", "")
	spwSecret := getEnv("SPW_CLIENT_SECRET", "")
	jwtSecret := getEnv("JWT_SECRET", "")
	playersAddr := getEnv("PLAYERS_ADDR", "players:50051")
	mcAddr := getEnv("MC_SERVER_ADDR", "spm.spworlds.org")
	port := getEnv("PORT", "8080")

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		l.Error("open db", slog.Any("err", err))
		return
	}
	defer db.Close()

	{
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			l.Error("ping db", slog.Any("err", err))
			return
		}
	}

	spwClient := spworlds.NewClient(spwID, spwSecret, nil)
	authp := auth.NewAuthProvider(&SPWmini{token: spwSecret}, spwClient, jwtSecret, l)

	grpcConn, err := grpc.NewClient(playersAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		l.Error("dial players", slog.Any("err", err))
		return
	}
	defer grpcConn.Close()

	playerClient := pb.NewPlayerServiceClient(grpcConn)
	repo := postgres.NewTransferRepo(db)
	service := transfers.NewTransferService(repo, spwClient, playerClient, l, mcAddr)
	handler := httpHandlers.NewHandler(authp, spwClient, service, l)

	mux := http.NewServeMux()
	handler.Routes(mux)

	eventListener := postgres.NewEventListener(dbURL)

	worker := transfers.NewWorker(service, l, transfers.WorkerConfig{
		PollInterval:  time.Second * 25,
		BatchLimit:    1,
		LeaseDuration: time.Minute * 2,
		Concurrency:   15,
	}, eventListener)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	workerErrCh := make(chan error, 1)
	go func() {
		l.Info("starting transfer worker")
		workerErrCh <- worker.Run(ctx)
	}()

	srv := &http.Server{Addr: ":" + port, Handler: mux}
	srvErrCh := make(chan error, 1)
	go func() {
		l.Info("starting transfers service", slog.String("addr", srv.Addr))
		srvErrCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		l.Info("shutdown signal received")
	case err := <-workerErrCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			l.Error("worker exited", slog.Any("err", err))
		}
	case err := <-srvErrCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			l.Error("server error", slog.Any("err", err))
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		l.Error("http shutdown", slog.Any("err", err))
	}

	stop()
	if err := <-workerErrCh; err != nil && !errors.Is(err, context.Canceled) {
		l.Error("worker stopped with error", slog.Any("err", err))
	}
}
