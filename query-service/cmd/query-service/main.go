package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prxssh/csv-ingestor/query-service/api"
	"github.com/prxssh/csv-ingestor/query-service/config"
	"github.com/prxssh/csv-ingestor/query-service/config/mongo"
	"github.com/prxssh/csv-ingestor/query-service/internal/movie"
	"github.com/prxssh/csv-ingestor/query-service/internal/utils/tracer"
	"golang.org/x/sync/errgroup"
)

const (
	startupTimeout  = 30 * time.Second
	shutdownTimeout = 30 * time.Second
)

func main() {
	if err := run(); err != nil {
		slog.Error("application fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	if err := config.LoadConfig(); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	setupLogger()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	tracerShutdown := tracer.NewOtelTracer(
		ctx,
		config.ServiceName,
		config.Env.OtelExporterURL,
		config.Env.OtelSamplingRate,
	)
	defer func() {
		if err := tracerShutdown(context.Background()); err != nil {
			slog.Error("failed to shutdown tracer cleanly", "error", err)
		}
	}()

	startupCtx, startupCancel := context.WithTimeout(ctx, startupTimeout)
	defer startupCancel()

	db, err := mongo.NewClient(startupCtx)
	if err != nil {
		return fmt.Errorf("mongo: %w", err)
	}
	defer func() {
		if err := db.Disconnect(context.Background()); err != nil {
			slog.Error("mongodb disconnect failed", "error", err)
		} else {
			slog.Info("mongodb disconnected cleanly")
		}
	}()

	// Services
	movieRepo := movie.NewRepository(db.Collection(mongo.CollMovies))
	movieSvc := movie.NewService(movieRepo)

	server, err := api.NewServer(ctx, &api.Deps{MovieService: movieSvc})
	if err != nil {
		return fmt.Errorf("api server: %w", err)
	}

	if startupCtx.Err() != nil {
		return fmt.Errorf("startup timeout exceeded")
	}
	startupCancel()

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		slog.Info("server listening", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		<-gCtx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("server shutdown failed", "error", err.Error())
		}

		slog.Info("shutdown success")
		return nil
	})

	if err := g.Wait(); err != nil {
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	}

	return nil
}

func setupLogger() {
	level := slog.LevelDebug
	if config.Env.IsProduction() {
		level = slog.LevelInfo
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     level,
		AddSource: false,
	})))
}
