package movie

import (
	"strings"
	"testing"
	"time"
)

func TestParseRow_HappyPath(t *testing.T) {
	record := []string{
		"30000000",              // budget
		"http://example.com",    // homepage
		"en",                    // original_language
		"Toy Story",             // original_title
		"A movie about toys.",   // overview
		"1995-10-30",            // release_date
		"373554033",             // revenue
		"81",                    // runtime
		"Released",              // status
		"Toy Story",             // title
		"7.7",                   // vote_average
		"5415",                  // vote_count
		"3",                     // production_company_id
		"16",                    // genre_id
		"['English', 'French']", // languages
	}

	m, err := ParseRow(record)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertEqual(t, "Budget", m.Budget, int64(30000000))
	assertEqual(t, "Homepage", m.Homepage, "http://example.com")
	assertEqual(t, "OriginalLanguage", m.OriginalLanguage, "en")
	assertEqual(t, "OriginalTitle", m.OriginalTitle, "Toy Story")
	assertEqual(t, "Overview", m.Overview, "A movie about toys.")
	assertEqual(t, "Revenue", m.Revenue, int64(373554033))
	assertEqual(t, "Runtime", m.Runtime, int32(81))
	assertEqual(t, "Status", m.Status, "Released")
	assertEqual(t, "Title", m.Title, "Toy Story")
	assertEqual(t, "VoteAverage", m.VoteAverage, 7.7)
	assertEqual(t, "VoteCount", m.VoteCount, int32(5415))
	assertEqual(t, "ProductionCompanyID", m.ProductionCompanyID, int32(3))
	assertEqual(t, "GenreID", m.GenreID, int32(16))
	assertEqual(t, "Year", m.Year, int32(1995))

	expectedDate := time.Date(1995, 10, 30, 0, 0, 0, 0, time.UTC)
	if !m.ReleaseDate.Equal(expectedDate) {
		t.Errorf("ReleaseDate: got %v, want %v", m.ReleaseDate, expectedDate)
	}

	if len(m.Languages) != 2 {
		t.Fatalf("Languages: got %d items, want 2", len(m.Languages))
	}
	assertEqual(t, "Languages[0]", m.Languages[0], "English")
	assertEqual(t, "Languages[1]", m.Languages[1], "French")
}

func TestParseRow_WhitespacePadding(t *testing.T) {
	record := makeRecord()
	record[colBudget] = "  30000000  "
	record[colOriginalTitle] = "  Padded Title  "
	record[colVoteAverage] = " 8.5 "

	m, err := ParseRow(record)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertEqual(t, "Budget", m.Budget, int64(30000000))
	assertEqual(t, "OriginalTitle", m.OriginalTitle, "Padded Title")
	assertEqual(t, "VoteAverage", m.VoteAverage, 8.5)
}

func TestParseRow_ExtraTrailingColumns(t *testing.T) {
	// Real CSV files often have trailing empty columns or extra data
	record := append(makeRecord(), "extra1", "extra2", "")
	_, err := ParseRow(record)
	if err != nil {
		t.Fatalf("should tolerate extra columns, got: %v", err)
	}
}

func TestParseRow_EmptyOptionalFields(t *testing.T) {
	record := []string{
		"", "", "en", "Test", "", "", "", "", "Released", "Test",
		"", "", "", "", "",
	}

	m, err := ParseRow(record)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertEqual(t, "Budget", m.Budget, int64(0))
	assertEqual(t, "Revenue", m.Revenue, int64(0))
	assertEqual(t, "Runtime", m.Runtime, int32(0))
	assertEqual(t, "VoteAverage", m.VoteAverage, 0.0)
	assertEqual(t, "VoteCount", m.VoteCount, int32(0))
	assertEqual(t, "Year", m.Year, int32(0))

	if !m.ReleaseDate.IsZero() {
		t.Errorf("ReleaseDate should be zero for empty input, got %v", m.ReleaseDate)
	}
	if m.Languages != nil {
		t.Errorf("Languages should be nil for empty input, got %v", m.Languages)
	}
}

func TestParseRow_TooFewColumns(t *testing.T) {
	_, err := ParseRow([]string{"only", "three", "columns"})
	if err == nil {
		t.Fatal("expected error for too few columns")
	}
	if !strings.Contains(err.Error(), "expected at least") {
		t.Errorf("error should mention column count, got: %v", err)
	}
}

func TestParseRow_ErrorMessages(t *testing.T) {
	tests := []struct {
		name      string
		col       int
		value     string
		wantInErr string
	}{
		{"invalid budget", colBudget, "xyz", "budget"},
		{"invalid revenue", colRevenue, "xyz", "revenue"},
		{"invalid runtime", colRuntime, "xyz", "runtime"},
		{"invalid vote_average", colVoteAverage, "xyz", "vote_average"},
		{"invalid vote_count", colVoteCount, "xyz", "vote_count"},
		{"invalid genre_id", colGenreID, "xyz", "genre_id"},
		{"invalid company_id", colProductionCompanyID, "xyz", "production_company_id"},
		{"invalid date", colReleaseDate, "not-a-date", "release_date"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := makeRecord()
			record[tt.col] = tt.value

			_, err := ParseRow(record)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantInErr) {
				t.Errorf("error should contain %q, got: %v", tt.wantInErr, err)
			}
		})
	}
}

func TestParseRow_LargeValues(t *testing.T) {
	record := makeRecord()
	record[colBudget] = "9999999999999" // exceeds int32 but fits int64
	record[colRevenue] = "9999999999999"

	m, err := ParseRow(record)
	if err != nil {
		t.Fatalf("large values should parse as int64: %v", err)
	}

	assertEqual(t, "Budget", m.Budget, int64(9999999999999))
	assertEqual(t, "Revenue", m.Revenue, int64(9999999999999))
}

func TestParseRow_ZeroBudgetAndRevenue(t *testing.T) {
	record := makeRecord()
	record[colBudget] = "0"
	record[colRevenue] = "0"
	record[colVoteAverage] = "0"

	m, err := ParseRow(record)
	if err != nil {
		t.Fatalf("zero values should be valid: %v", err)
	}

	assertEqual(t, "Budget", m.Budget, int64(0))
	assertEqual(t, "Revenue", m.Revenue, int64(0))
	assertEqual(t, "VoteAverage", m.VoteAverage, 0.0)
}

func TestParseLanguages(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{"python list", "['English', 'French']", []string{"English", "French"}},
		{"single language", "['English']", []string{"English"}},
		{"empty string", "", nil},
		{"empty brackets", "[]", nil},
		{
			"three languages",
			"['English', 'Français', 'Deutsch']",
			[]string{"English", "Français", "Deutsch"},
		},
		{"no quotes", "[English]", nil}, // doesn't match regex pattern
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLanguages(tt.raw)
			if len(got) != len(tt.want) {
				t.Fatalf(
					"got %d languages, want %d: %v",
					len(got),
					len(tt.want),
					got,
				)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf(
						"language[%d]: got %q, want %q",
						i,
						got[i],
						tt.want[i],
					)
				}
			}
		})
	}
}

func TestParseDate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantYear int32
		wantErr  bool
	}{
		{"valid date", "1995-10-30", 1995, false},
		{"empty string", "", 0, false},
		{"invalid format dd-mm-yyyy", "30-10-1995", 0, true},
		{"garbage", "hello", 0, true},
		{"partial date", "1995-10", 0, true},
		{"future date", "2099-12-31", 2099, false},
		{"leap year", "2000-02-29", 2000, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			date, year, err := parseDate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				assertEqual(t, "year", year, tt.wantYear)
				if tt.input == "" && !date.IsZero() {
					t.Errorf("expected zero date for empty input")
				}
				if tt.input != "" && date.IsZero() {
					t.Errorf("expected non-zero date for input %q", tt.input)
				}
			}
		})
	}
}

// --- helpers ---

func makeRecord() []string {
	return []string{
		"30000000", "http://example.com", "en", "Test Movie", "Overview",
		"2000-01-01", "100000000", "120", "Released", "Test Movie",
		"7.5", "1000", "1", "1", "['English']",
	}
}

func assertEqual[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", name, got, want)
	}
}
