package config

import (
	"fmt"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Addr     string `env:"TELEGRAM_BOT_TOKEN"`
	Port     string `env:"TELEGRAM_CHAT_ID"`
	LogLevel string `env:"LOG_LEVEL" env-default:"info"`
}

func Init() (*Config, error) {
	var cfg Config

	err := cleanenv.ReadConfig(".env", &cfg)

	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	return &cfg, nil
}
