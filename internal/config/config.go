package config

import (
	"fmt"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Token       string `env:"TELEGRAM_BOT_TOKEN"`
	ChatID      string `env:"TELEGRAM_CHAT_ID"`
	LogLevel    string `env:"LOG_LEVEL" env-default:"info"`
	MetricsPort string `env:"METRICS_PORT" env-default:"8080"`
}

func Init() (*Config, error) {
	var cfg Config

	if _, err := os.Stat(".env"); err == nil {
		if err := cleanenv.ReadConfig(".env", &cfg); err != nil {
			return nil, fmt.Errorf("failed to read .env file: %w", err)
		}
	} else {
		if err := cleanenv.ReadEnv(&cfg); err != nil {
			return nil, fmt.Errorf("failed to read environment variables: %w", err)
		}
	}

	return &cfg, nil
}
