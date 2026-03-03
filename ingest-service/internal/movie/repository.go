package movie

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Repository struct {
	coll *mongo.Collection
}

func NewRepository(coll *mongo.Collection) *Repository {
	return &Repository{coll: coll}
}

func (r *Repository) batchUpsert(
	ctx context.Context,
	movies []Movie,
) (inserted, modified int64, err error) {
	if len(movies) == 0 {
		return 0, 0, nil
	}

	models := make([]mongo.WriteModel, 0, len(movies))
	now := time.Now().UTC()

	for i := range movies {
		movies[i].UpdatedAt = now

		filter := bson.M{
			"original_title": movies[i].OriginalTitle,
			"release_date":   movies[i].ReleaseDate,
		}

		m := movies[i]
		update := bson.M{
			"$set": bson.M{
				"budget":                m.Budget,
				"homepage":              m.Homepage,
				"original_language":     m.OriginalLanguage,
				"original_title":        m.OriginalTitle,
				"overview":              m.Overview,
				"release_date":          m.ReleaseDate,
				"year":                  m.Year,
				"revenue":               m.Revenue,
				"runtime":               m.Runtime,
				"status":                m.Status,
				"title":                 m.Title,
				"vote_average":          m.VoteAverage,
				"vote_count":            m.VoteCount,
				"production_company_id": m.ProductionCompanyID,
				"genre_id":              m.GenreID,
				"languages":             m.Languages,
				"updated_at":            now,
			},
			"$setOnInsert": bson.M{"created_at": now},
		}

		models = append(models, mongo.NewUpdateOneModel().
			SetFilter(filter).
			SetUpdate(update).
			SetUpsert(true),
		)
	}

	opts := options.BulkWrite().SetOrdered(false)
	result, err := r.coll.BulkWrite(ctx, models, opts)
	if err != nil {
		return 0, 0, fmt.Errorf("batch upsert movies: %w", err)
	}

	return result.UpsertedCount + result.InsertedCount, result.ModifiedCount, nil
}

func (r *Repository) createIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		// Unique — prevents duplicate movies on re-upload
		{
			Keys: bson.D{
				{Key: "original_title", Value: 1},
				{Key: "release_date", Value: 1},
			},
			Options: options.Index().SetName("idx_title_release").SetUnique(true),
		},
		// Filter by year
		{
			Keys:    bson.D{{Key: "year", Value: 1}},
			Options: options.Index().SetName("idx_year"),
		},
		// Filter by language (multikey)
		{
			Keys:    bson.D{{Key: "languages", Value: 1}},
			Options: options.Index().SetName("idx_languages"),
		},
		// Compound: year + language + release_date (covers filter+sort)
		{
			Keys: bson.D{
				{Key: "year", Value: 1},
				{Key: "languages", Value: 1},
				{Key: "release_date", Value: -1},
			},
			Options: options.Index().SetName("idx_year_lang_date"),
		},
		// Compound: year + language + vote_average (covers filter+sort)
		{
			Keys: bson.D{
				{Key: "year", Value: 1},
				{Key: "languages", Value: 1},
				{Key: "vote_average", Value: -1},
			},
			Options: options.Index().SetName("idx_year_lang_rating"),
		},
	}

	_, err := r.coll.Indexes().CreateMany(ctx, indexes)
	return err
}
