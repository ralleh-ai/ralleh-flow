package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ralleh-ai/ralleh-flow/apps/api/internal/api"
	"github.com/ralleh-ai/ralleh-flow/apps/api/internal/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	zerolog.TimeFieldFormat = time.RFC3339

	cfg := config.Load()
	app := api.NewApp(cfg)
	server := app.Server

	runCtx, runCancel := context.WithCancel(context.Background())
	defer runCancel()
	go app.Orchestrator.Start(runCtx)

	go func() {
		log.Info().Str("addr", cfg.APIAddr).Msg("ralleh-flow api listening")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server failed")
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	runCancel()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("graceful shutdown failed")
	}
}
