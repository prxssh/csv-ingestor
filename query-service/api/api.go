package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	movieapi "github.com/prxssh/csv-ingestor/query-service/api/movieapi"
	"github.com/prxssh/csv-ingestor/query-service/config"
	"github.com/prxssh/csv-ingestor/query-service/internal/movie"
	"github.com/prxssh/csv-ingestor/query-service/internal/utils/apiutil"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

type Deps struct {
	MovieService *movie.Service
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

	movieapi.NewHandler(deps.MovieService).InitV1Routes(r.Group("/v1/movies"))

	return &http.Server{
		Handler:           r,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		Addr:              fmt.Sprintf(":%s", config.Env.Port),
	}, nil
}
