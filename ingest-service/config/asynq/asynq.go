package asynqcfg

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/hibiken/asynq"
	"github.com/prxssh/csv-ingestor/ingest-service/config"
)

// NewServer creates an asynq.Server configured for this service.
func NewServer() (*asynq.Server, error) {
	redisOpt, err := asynq.ParseRedisURI(config.Env.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}

	srv := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: 4,
		Queues:      map[string]int{"default": 1},
		Logger:      newLogger(),
	})

	return srv, nil
}

// NewClient creates an asynq.Client for enqueuing tasks.
func NewClient() (*asynq.Client, error) {
	redisOpt, err := asynq.ParseRedisURI(config.Env.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}

	return asynq.NewClient(redisOpt), nil
}

// logger adapts slog to asynq's logger interface.
type logger struct{}

func newLogger() *logger { return &logger{} }

func (l *logger) Debug(args ...interface{}) { slog.Debug(fmt.Sprint(args...)) }
func (l *logger) Info(args ...interface{})  { slog.Info(fmt.Sprint(args...)) }
func (l *logger) Warn(args ...interface{})  { slog.Warn(fmt.Sprint(args...)) }
func (l *logger) Error(args ...interface{}) { slog.Error(fmt.Sprint(args...)) }
func (l *logger) Fatal(args ...interface{}) {
	slog.Error(fmt.Sprint(args...))
	os.Exit(1)
}
