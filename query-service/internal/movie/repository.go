package movie

import (
	"context"
	"fmt"
	"math"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Repository struct {
	coll *mongo.Collection
}

func NewRepository(coll *mongo.Collection) *Repository {
	return &Repository{coll: coll}
}

func (r *Repository) findByID(ctx context.Context, id string) (*Movie, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, ErrInvalidMovieID
	}

	var movie Movie
	if err := r.coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&movie); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("find movie: %w", err)
	}
	return &movie, nil
}

func (r *Repository) list(ctx context.Context, filter ListMoviesFilter) (*ListMoviesResult, error) {
	query := bson.M{}

	if filter.Year != nil {
		query["year"] = *filter.Year
	}
	if filter.Language != nil {
		query["languages"] = *filter.Language
	}

	var total int64
	var err error
	if len(query) == 0 {
		total, err = r.coll.EstimatedDocumentCount(ctx)
	} else {
		total, err = r.coll.CountDocuments(ctx, query)
	}
	if err != nil {
		return nil, fmt.Errorf("count movies: %w", err)
	}

	totalPages := int64(math.Ceil(float64(total) / float64(filter.Limit)))
	sortField := filter.SortBy.MongoField()
	sortDir := filter.SortDir.MongoDir()

	skip := (filter.Page - 1) * filter.Limit

	opts := options.Find().
		SetSort(bson.D{{Key: sortField, Value: sortDir}}).
		SetSkip(skip).
		SetLimit(filter.Limit)

	cursor, err := r.coll.Find(ctx, query, opts)
	if err != nil {
		return nil, fmt.Errorf("find movies: %w", err)
	}
	defer cursor.Close(ctx)

	movies := make([]Movie, 0)
	if err := cursor.All(ctx, &movies); err != nil {
		return nil, fmt.Errorf("decode movies: %w", err)
	}

	return &ListMoviesResult{
		Movies:     movies,
		Total:      total,
		Page:       filter.Page,
		Limit:      filter.Limit,
		TotalPages: totalPages,
	}, nil
}
