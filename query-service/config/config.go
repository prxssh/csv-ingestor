package config

import (
	"github.com/kelseyhightower/envconfig"
)

const ServiceName = "query-service"

type env struct {
	DatabaseURL                string  `required:"true" split_words:"true"`
	DatabasePoolMaxConnections int32   `                split_words:"true" default:"2"`
	DatabasePoolMinConnections int32   `                split_words:"true" default:"1"`
	Environment                string  `                split_words:"true" default:"dev"`
	OtelExporterURL            string  `required:"true"                                   envconfig:"OTEL_EXPORTER_URL"`
	OtelSamplingRate           float64 `                                   default:"0.02" envconfig:"OTEL_SAMPLING_RATE"`

	Port string `default:"6969"`
}

var Env env

func LoadConfig() error {
	return envconfig.Process("", &Env)
}

func (e *env) IsProduction() bool {
	return e.Environment == "production"
}
