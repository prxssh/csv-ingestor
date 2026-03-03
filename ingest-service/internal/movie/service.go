package movie

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// CSV header → struct field index (0-based).
const (
	colBudget              = 0
	colHomepage            = 1
	colOriginalLanguage    = 2
	colOriginalTitle       = 3
	colOverview            = 4
	colReleaseDate         = 5
	colRevenue             = 6
	colRuntime             = 7
	colStatus              = 8
	colTitle               = 9
	colVoteAverage         = 10
	colVoteCount           = 11
	colProductionCompanyID = 12
	colGenreID             = 13
	colLanguages           = 14
	expectedColumns        = 15
)

var languagesRe = regexp.MustCompile(`'([^']*)'`)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateIndexes(ctx context.Context) error {
	return s.repo.createIndexes(ctx)
}

func (s *Service) BatchUpsert(
	ctx context.Context,
	movies []Movie,
) (inserted, modified int64, err error) {
	return s.repo.batchUpsert(ctx, movies)
}

func ParseRow(record []string) (Movie, error) {
	if len(record) < expectedColumns {
		return Movie{}, fmt.Errorf(
			"expected at least %d columns, got %d",
			expectedColumns,
			len(record),
		)
	}

	budget, err := parseInt64(record[colBudget])
	if err != nil {
		return Movie{}, fmt.Errorf("budget: %w", err)
	}

	revenue, err := parseInt64(record[colRevenue])
	if err != nil {
		return Movie{}, fmt.Errorf("revenue: %w", err)
	}

	runtime, err := parseInt32(record[colRuntime])
	if err != nil {
		return Movie{}, fmt.Errorf("runtime: %w", err)
	}

	voteAvg, err := parseFloat64(record[colVoteAverage])
	if err != nil {
		return Movie{}, fmt.Errorf("vote_average: %w", err)
	}

	voteCnt, err := parseInt32(record[colVoteCount])
	if err != nil {
		return Movie{}, fmt.Errorf("vote_count: %w", err)
	}

	companyID, err := parseInt32(record[colProductionCompanyID])
	if err != nil {
		return Movie{}, fmt.Errorf("production_company_id: %w", err)
	}

	genreID, err := parseInt32(record[colGenreID])
	if err != nil {
		return Movie{}, fmt.Errorf("genre_id: %w", err)
	}

	releaseDate, year, err := parseDate(record[colReleaseDate])
	if err != nil {
		return Movie{}, fmt.Errorf("release_date: %w", err)
	}

	return Movie{
		Budget:              budget,
		Homepage:            strings.TrimSpace(record[colHomepage]),
		OriginalLanguage:    strings.TrimSpace(record[colOriginalLanguage]),
		OriginalTitle:       strings.TrimSpace(record[colOriginalTitle]),
		Overview:            strings.TrimSpace(record[colOverview]),
		ReleaseDate:         releaseDate,
		Year:                year,
		Revenue:             revenue,
		Runtime:             runtime,
		Status:              strings.TrimSpace(record[colStatus]),
		Title:               strings.TrimSpace(record[colTitle]),
		VoteAverage:         voteAvg,
		VoteCount:           voteCnt,
		ProductionCompanyID: companyID,
		GenreID:             genreID,
		Languages:           parseLanguages(record[colLanguages]),
	}, nil
}

func parseDate(s string) (time.Time, int32, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, 0, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("invalid date %q: %w", s, err)
	}
	return t, int32(t.Year()), nil
}

func parseLanguages(raw string) []string {
	matches := languagesRe.FindAllStringSubmatch(raw, -1)
	if len(matches) == 0 {
		return nil
	}
	langs := make([]string, 0, len(matches))
	for _, m := range matches {
		langs = append(langs, m[1])
	}
	return langs
}

func parseInt64(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	return strconv.ParseInt(s, 10, 64)
}

func parseInt32(s string) (int32, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	v, err := strconv.ParseInt(s, 10, 32)
	return int32(v), err
}

func parseFloat64(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	return strconv.ParseFloat(s, 64)
}
