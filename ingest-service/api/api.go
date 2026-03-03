package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/prxssh/csv-ingestor/ingest-service/api/uploadapi"
	"github.com/prxssh/csv-ingestor/ingest-service/config"
	"github.com/prxssh/csv-ingestor/ingest-service/internal/upload"
	"github.com/prxssh/csv-ingestor/ingest-service/internal/utils/apiutil"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

type Deps struct {
	UploadService *upload.Service
	AsynqClient   *asynq.Client
}

func NewServer(ctx context.Context, deps *Deps) (*http.Server, error) {
	if config.Env.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(otelgin.Middleware(config.ServiceName))

	r.GET("/ping", func(ctx *gin.Context) {
		apiutil.Success(ctx, http.StatusOK, "PONG")
	})

	uploadapi.NewHandler(deps.UploadService, deps.AsynqClient).
		InitV1Routes(r.Group("/v1/uploads"))

	return &http.Server{
		Handler:           r,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		Addr:              fmt.Sprintf(":%s", config.Env.Port),
	}, nil
}
