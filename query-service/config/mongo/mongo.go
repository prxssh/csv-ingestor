package mongo

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/prxssh/csv-ingestor/query-service/config"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
	"go.opentelemetry.io/contrib/instrumentation/go.mongodb.org/mongo-driver/mongo/otelmongo"
)

const (
	CollMovies     = "movies"
	CollUploadJobs = "upload_jobs"
)

type DB struct {
	client *mongo.Client
	db     *mongo.Database
}

func NewClient(ctx context.Context) (*DB, error) {
	opts := options.Client().
		ApplyURI(config.Env.DatabaseURL).
		SetReadPreference(readpref.SecondaryPreferred()).
		SetMaxPoolSize(uint64(config.Env.DatabasePoolMaxConnections)).
		SetMinPoolSize(uint64(config.Env.DatabasePoolMinConnections)).
		SetMaxConnIdleTime(30 * time.Second).
		SetConnectTimeout(10 * time.Second).
		SetServerSelectionTimeout(10 * time.Second).
		SetMonitor(otelmongo.NewMonitor())

	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return nil, err
	}

	dbName, err := dbNameFromURI(config.Env.DatabaseURL)
	if err != nil {
		return nil, err
	}
	d := &DB{
		client: client,
		db:     client.Database(dbName),
	}
	if err := d.createIndexes(ctx); err != nil {
		slog.Warn("failed to create indexes", "error", err)
	}

	return d, nil
}

func (d *DB) Collection(name string) *mongo.Collection {
	return d.db.Collection(name)
}

func (d *DB) Disconnect(ctx context.Context) error {
	return d.client.Disconnect(ctx)
}

func (d *DB) createIndexes(ctx context.Context) error {
	return d.createMovieIndexes(ctx)
}

func (d *DB) createMovieIndexes(ctx context.Context) error {
	coll := d.db.Collection(CollMovies)

	indexes := []mongo.IndexModel{
		// Unique — prevents duplicate movies on re-upload
		{
			Keys: bson.D{
				bson.E{Key: "title", Value: 1},
				bson.E{Key: "release_date", Value: 1},
			},
			Options: options.Index().
				SetUnique(true).
				SetName("idx_title_release_date"),
		},
		// Filter by year
		{
			Keys:    bson.D{bson.E{Key: "year", Value: 1}},
			Options: options.Index().SetName("idx_year"),
		},
		// Filter by language
		{
			Keys:    bson.D{bson.E{Key: "languages", Value: 1}},
			Options: options.Index().SetName("idx_languages"),
		},
		// Compound — year + language + release_date
		{
			Keys: bson.D{
				bson.E{Key: "year", Value: 1},
				bson.E{Key: "languages", Value: 1},
				bson.E{Key: "release_date", Value: -1},
			},
			Options: options.Index().SetName("idx_year_lang_date"),
		},
		// Compound — year + language + vote_average
		{
			Keys: bson.D{
				bson.E{Key: "year", Value: 1},
				bson.E{Key: "languages", Value: 1},
				bson.E{Key: "vote_average", Value: -1},
			},
			Options: options.Index().SetName("idx_year_lang_rating"),
		},
	}

	_, err := coll.Indexes().CreateMany(ctx, indexes)
	return err
}

func dbNameFromURI(uri string) (string, error) {
	cs, err := connstring.ParseAndValidate(uri)
	if err != nil || cs.Database == "" {
		return "", fmt.Errorf("invalid database")
	}

	return cs.Database, nil
}
