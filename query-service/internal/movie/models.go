package movie

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Movie struct {
	ID                  primitive.ObjectID `bson:"_id,omitempty"         json:"id"`
	Budget              int64              `bson:"budget"                json:"budget"`
	Homepage            string             `bson:"homepage,omitempty"    json:"homepage,omitempty"`
	OriginalLanguage    string             `bson:"original_language"     json:"original_language"`
	OriginalTitle       string             `bson:"original_title"        json:"original_title"`
	Overview            string             `bson:"overview,omitempty"    json:"overview,omitempty"`
	ReleaseDate         time.Time          `bson:"release_date"          json:"release_date"`
	Year                int32              `bson:"year"                  json:"year"`
	Revenue             int64              `bson:"revenue"               json:"revenue"`
	Runtime             int32              `bson:"runtime"               json:"runtime"`
	Status              string             `bson:"status"                json:"status"`
	Title               string             `bson:"title"                 json:"title"`
	VoteAverage         float64            `bson:"vote_average"          json:"vote_average"`
	VoteCount           int32              `bson:"vote_count"            json:"vote_count"`
	ProductionCompanyID int32              `bson:"production_company_id" json:"production_company_id"`
	GenreID             int32              `bson:"genre_id"              json:"genre_id"`
	Languages           []string           `bson:"languages"             json:"languages"`
	CreatedAt           time.Time          `bson:"created_at"            json:"created_at"`
	UpdatedAt           time.Time          `bson:"updated_at"            json:"updated_at"`
}

type SortField int

const (
	SortByReleaseDate SortField = iota
	SortByVoteAverage
)

func (s SortField) MongoField() string {
	switch s {
	case SortByVoteAverage:
		return "vote_average"
	default:
		return "release_date"
	}
}

type SortDirection int

const (
	SortDesc SortDirection = iota
	SortAsc
)

func (d SortDirection) MongoDir() int {
	if d == SortAsc {
		return 1
	}
	return -1
}

type ListMoviesFilter struct {
	Year     *int32
	Language *string
	SortBy   SortField
	SortDir  SortDirection
	Page     int64
	Limit    int64
}

type ListMoviesResult struct {
	Movies     []Movie `json:"movies"`
	Total      int64   `json:"total"`
	Page       int64   `json:"page"`
	Limit      int64   `json:"limit"`
	TotalPages int64   `json:"total_pages"`
}
