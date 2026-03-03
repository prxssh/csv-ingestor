package config

import (
	"github.com/kelseyhightower/envconfig"
)

const ServiceName = "ingest-service"

type env struct {
	DatabaseURL                string  `required:"true" split_words:"true"`
	DatabasePoolMaxConnections int32   `                split_words:"true" default:"2"`
	DatabasePoolMinConnections int32   `                split_words:"true" default:"1"`
	Environment                string  `                split_words:"true" default:"dev"`
	OtelExporterURL            string  `required:"true"                                   envconfig:"OTEL_EXPORTER_URL"`
	OtelSamplingRate           float64 `                                   default:"0.02" envconfig:"OTEL_SAMPLING_RATE"`
	Port                       string  `                                   default:"6970"`
	RedisURL                   string  `required:"true" split_words:"true"`
	S3Bucket                   string  `required:"true"                                   envconfig:"S3_BUCKET"`
	S3Region                   string  `required:"true"                                   envconfig:"S3_REGION"`
	S3Endpoint                 string  `                                                  envconfig:"S3_ENDPOINT"`
	S3AccessKeyID              string  `required:"true"                                   envconfig:"S3_ACCESS_KEY_ID"`
	S3SecretAccessKey          string  `required:"true"                                   envconfig:"S3_SECRET_ACCESS_KEY"`
	S3PresignTTLMins           int     `                                   default:"60"   envconfig:"S3_PRESIGN_TTL_MINS"`
}

var Env env

func LoadConfig() error {
	return envconfig.Process("", &Env)
}

func (e *env) IsProduction() bool {
	return e.Environment == "production"
}
