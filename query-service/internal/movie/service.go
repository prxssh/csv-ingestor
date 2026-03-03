package movie

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "github.com/prxssh/csv-ingestor/query-service/internal/movie"

// Service is the public API for the movie domain in the query-service.
type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListMovies(
	ctx context.Context,
	filter ListMoviesFilter,
) (*ListMoviesResult, error) {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "movie.ListMovies",
		trace.WithAttributes(
			attribute.Int64("page", filter.Page),
			attribute.Int64("page_size", filter.Limit),
			attribute.String("sort_by", filter.SortBy.MongoField()),
		),
	)
	defer span.End()

	if filter.Year != nil {
		span.SetAttributes(attribute.Int("filter.year", int(*filter.Year)))
	}
	if filter.Language != nil {
		span.SetAttributes(attribute.String("filter.language", *filter.Language))
	}

	return s.repo.list(ctx, filter)
}

func (s *Service) GetMovie(ctx context.Context, id string) (*Movie, error) {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "movie.GetMovie",
		trace.WithAttributes(attribute.String("movie.id", id)),
	)
	defer span.End()

	return s.repo.findByID(ctx, id)
}
