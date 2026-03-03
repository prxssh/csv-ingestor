package movieapi

import (
	"testing"

	"github.com/prxssh/csv-ingestor/query-service/internal/movie"
)

func TestListMoviesQuery_Defaults(t *testing.T) {
	q := listMoviesQuery{}
	q.defaults()

	if q.Page != 1 {
		t.Errorf("Page: got %d, want 1", q.Page)
	}
	if q.Limit != 20 {
		t.Errorf("Limit: got %d, want 20", q.Limit)
	}
	if q.SortBy != "release_date" {
		t.Errorf("SortBy: got %q, want %q", q.SortBy, "release_date")
	}
	if q.SortDir != "desc" {
		t.Errorf("SortDir: got %q, want %q", q.SortDir, "desc")
	}
}

func TestListMoviesQuery_Defaults_PreservesExisting(t *testing.T) {
	q := listMoviesQuery{
		Page:    5,
		Limit:   50,
		SortBy:  "vote_average",
		SortDir: "asc",
	}
	q.defaults()

	if q.Page != 5 {
		t.Errorf("Page: got %d, want 5", q.Page)
	}
	if q.Limit != 50 {
		t.Errorf("Limit: got %d, want 50", q.Limit)
	}
	if q.SortBy != "vote_average" {
		t.Errorf("SortBy: got %q, want %q", q.SortBy, "vote_average")
	}
	if q.SortDir != "asc" {
		t.Errorf("SortDir: got %q, want %q", q.SortDir, "asc")
	}
}

func TestListMoviesQuery_SortField(t *testing.T) {
	tests := []struct {
		input string
		want  movie.SortField
	}{
		{"release_date", movie.SortByReleaseDate},
		{"vote_average", movie.SortByVoteAverage},
		{"unknown", movie.SortByReleaseDate},
		{"", movie.SortByReleaseDate},
	}

	for _, tt := range tests {
		q := listMoviesQuery{SortBy: tt.input}
		got := q.sortField()
		if got != tt.want {
			t.Errorf("sortField(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestListMoviesQuery_SortDirection(t *testing.T) {
	tests := []struct {
		input string
		want  movie.SortDirection
	}{
		{"asc", movie.SortAsc},
		{"desc", movie.SortDesc},
		{"", movie.SortDesc},
		{"invalid", movie.SortDesc},
	}

	for _, tt := range tests {
		q := listMoviesQuery{SortDir: tt.input}
		got := q.sortDirection()
		if got != tt.want {
			t.Errorf("sortDirection(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
