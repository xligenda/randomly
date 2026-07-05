package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
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

// todo: start worker looking for free transfers

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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		l.Error("ping db", slog.Any("err", err))
		return
	}

	spwClient := spworlds.NewClient(spwID, spwSecret, nil)

	authp := auth.NewAuthProvider(&SPWmini{
		token: spwSecret,
	}, spwClient, jwtSecret, l)

	conn, err := grpc.NewClient(playersAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		l.Error("dial players", slog.Any("err", err))
		return
	}
	defer conn.Close()

	playerClient := pb.NewPlayerServiceClient(conn)

	repo := postgres.NewTransferRepo(db)
	service := transfers.NewTransferService(repo, spwClient, playerClient, l, mcAddr)

	handler := httpHandlers.NewHandler(authp, spwClient, service, l)

	mux := http.NewServeMux()
	handler.Routes(mux)

	srv := &http.Server{Addr: ":" + port, Handler: mux}
	l.Info("starting transfers service", slog.String("addr", srv.Addr))
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		l.Error("server error", slog.Any("err", err))
	}
}
