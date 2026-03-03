package movie

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Movie represents a single movie document in the movies collection.
type Movie struct {
	ID                  primitive.ObjectID `bson:"_id,omitempty"`
	Budget              int64              `bson:"budget"`
	Homepage            string             `bson:"homepage,omitempty"`
	OriginalLanguage    string             `bson:"original_language"`
	OriginalTitle       string             `bson:"original_title"`
	Overview            string             `bson:"overview,omitempty"`
	ReleaseDate         time.Time          `bson:"release_date"`
	Year                int32              `bson:"year"`
	Revenue             int64              `bson:"revenue"`
	Runtime             int32              `bson:"runtime"`
	Status              string             `bson:"status"`
	Title               string             `bson:"title"`
	VoteAverage         float64            `bson:"vote_average"`
	VoteCount           int32              `bson:"vote_count"`
	ProductionCompanyID int32              `bson:"production_company_id"`
	GenreID             int32              `bson:"genre_id"`
	Languages           []string           `bson:"languages"`
	CreatedAt           time.Time          `bson:"created_at"`
	UpdatedAt           time.Time          `bson:"updated_at"`
}
