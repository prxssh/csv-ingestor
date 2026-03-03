package mongo

import (
	"context"
	"fmt"
	"time"

	"github.com/prxssh/csv-ingestor/ingest-service/config"
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
		SetReadPreference(readpref.Nearest()).
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
	return d, nil
}

func (d *DB) Collection(name string) *mongo.Collection {
	return d.db.Collection(name)
}

func (d *DB) Disconnect(ctx context.Context) error {
	return d.client.Disconnect(ctx)
}

func dbNameFromURI(uri string) (string, error) {
	cs, err := connstring.ParseAndValidate(uri)
	if err != nil || cs.Database == "" {
		return "", fmt.Errorf("invalid database")
	}

	return cs.Database, nil
}
