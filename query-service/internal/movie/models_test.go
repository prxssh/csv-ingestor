package movie

import (
	"testing"
)

func TestSortField_MongoField(t *testing.T) {
	tests := []struct {
		field SortField
		want  string
	}{
		{SortByReleaseDate, "release_date"},
		{SortByVoteAverage, "vote_average"},
		{SortField(99), "release_date"}, // unknown defaults to release_date
	}

	for _, tt := range tests {
		got := tt.field.MongoField()
		if got != tt.want {
			t.Errorf("SortField(%d).MongoField() = %q, want %q", tt.field, got, tt.want)
		}
	}
}

func TestSortDirection_MongoDir(t *testing.T) {
	tests := []struct {
		dir  SortDirection
		want int
	}{
		{SortDesc, -1},
		{SortAsc, 1},
		{SortDirection(99), -1}, // unknown defaults to desc
	}

	for _, tt := range tests {
		got := tt.dir.MongoDir()
		if got != tt.want {
			t.Errorf("SortDirection(%d).MongoDir() = %d, want %d", tt.dir, got, tt.want)
		}
	}
}
