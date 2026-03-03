package movieapi

import "github.com/prxssh/csv-ingestor/query-service/internal/movie"

type movieIDURI struct {
	ID string `uri:"id" binding:"required"`
}

type listMoviesQuery struct {
	Page     int64  `form:"page"     binding:"omitempty,min=1"`
	Limit    int64  `form:"limit"    binding:"omitempty,min=1,max=100"`
	Year     *int32 `form:"year"     binding:"omitempty"`
	Language string `form:"language" binding:"omitempty"`
	SortBy   string `form:"sort_by"  binding:"omitempty,oneof=release_date vote_average"`
	SortDir  string `form:"sort_dir" binding:"omitempty,oneof=asc desc"`
}

func (q *listMoviesQuery) defaults() {
	if q.Page == 0 {
		q.Page = 1
	}
	if q.Limit == 0 {
		q.Limit = 20
	}
	if q.SortBy == "" {
		q.SortBy = "release_date"
	}
	if q.SortDir == "" {
		q.SortDir = "desc"
	}
}

func (q *listMoviesQuery) sortField() movie.SortField {
	if q.SortBy == "vote_average" {
		return movie.SortByVoteAverage
	}
	return movie.SortByReleaseDate
}

func (q *listMoviesQuery) sortDirection() movie.SortDirection {
	if q.SortDir == "asc" {
		return movie.SortAsc
	}
	return movie.SortDesc
}
