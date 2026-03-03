package movieapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prxssh/csv-ingestor/query-service/internal/movie"
	"github.com/prxssh/csv-ingestor/query-service/internal/utils/apiutil"
)

type Handler struct {
	movieSvc *movie.Service
}

func NewHandler(movieSvc *movie.Service) *Handler {
	return &Handler{movieSvc: movieSvc}
}

func (h *Handler) ListMovies(ctx *gin.Context) {
	var query listMoviesQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		apiutil.Error(ctx, http.StatusBadRequest, err.Error())
		return
	}
	query.defaults()

	filter := movie.ListMoviesFilter{
		Page:    query.Page,
		Limit:   query.Limit,
		Year:    query.Year,
		SortBy:  query.sortField(),
		SortDir: query.sortDirection(),
	}
	if query.Language != "" {
		filter.Language = &query.Language
	}

	result, err := h.movieSvc.ListMovies(ctx.Request.Context(), filter)
	if err != nil {
		apiutil.InternalError(ctx)
		return
	}

	apiutil.Success(ctx, http.StatusOK, result)
}

func (h *Handler) GetMovie(ctx *gin.Context) {
	var uri movieIDURI
	if err := ctx.ShouldBindUri(&uri); err != nil {
		apiutil.Error(ctx, http.StatusBadRequest, err.Error())
		return
	}

	m, err := h.movieSvc.GetMovie(ctx.Request.Context(), uri.ID)
	if err != nil {
		apiutil.Error(ctx, http.StatusBadRequest, "invalid movie id")
		return
	}
	if m == nil {
		apiutil.Error(ctx, http.StatusNotFound, "movie not found")
		return
	}

	apiutil.Success(ctx, http.StatusOK, m)
}
