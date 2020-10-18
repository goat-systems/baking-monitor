package config

import (
	"github.com/caarlos0/env/v6"
	"github.com/go-playground/validator"
	"github.com/pkg/errors"
)

// Config encapsulates all configuration possibilities into a single structure
type Config struct {
	Baker    string `env:"BAKING_MONITOR_BAKER" validate:"required"`
	TezosAPI string `env:"BAKING_MONITOR_TEZOS_API" validate:"required"`
	Twilio   Twilio
}

// Twilio contains twilio API information for automatic notifications
type Twilio struct {
	AccountSID string   `env:"BAKING_MONITOR_ACCOUNT_SID" validate:"required"`
	AuthToken  string   `env:"BAKING_MONITOR_AUTH_TOKEN" validate:"required"`
	From       string   `env:"BAKING_MONITOR_FROM" validate:"required"`
	To         []string `env:"BAKING_MONITOR_TO" envSeparator:"," validate:"required"`
}

// New loads enviroment variables into a Config struct
func New() (Config, error) {
	config := Config{}
	if err := env.Parse(&config); err != nil {
		return config, errors.Wrap(err, "failed to load enviroment variables")
	}

	err := validator.New().Struct(&config)
	if err != nil {
		return config, errors.Wrap(err, "invalid input")
	}

	return config, nil
}
