package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Channels       []string `env:"NOTIFICATION_CHANNELS" env-required:"true" env-separator:","`
	Token          string   `env:"TELEGRAM_BOT_TOKEN"`
	ChatID         string   `env:"TELEGRAM_CHAT_ID"`
	SlackWebhook   string   `env:"SLACK_WEBHOOK"`
	DiscordWebhook string   `env:"DISCORD_WEBHOOK"`
	LogLevel       string   `env:"LOG_LEVEL" env-default:"info"`
	MetricsPort    string   `env:"METRICS_PORT" env-default:"8080"`
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

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate config: %w", err)
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	if _, err := strconv.Atoi(c.MetricsPort); err != nil {
		return fmt.Errorf("invalid METRICS_PORT %q: %w", c.MetricsPort, err)
	}

	cleanChannels := make([]string, 0, 3)
	seen := make(map[string]bool)

	for _, v := range c.Channels {
		ch := strings.ToLower(strings.TrimSpace(v))

		if ch == "" {
			continue
		}

		if seen[ch] {
			return fmt.Errorf("duplicate notification channel specified: %q", ch)
		}
		seen[ch] = true

		switch ch {
		case "telegram":
			if c.Token == "" || c.ChatID == "" {
				return fmt.Errorf("channel 'telegram' requires TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID")
			}
		case "slack":
			if c.SlackWebhook == "" {
				return fmt.Errorf("channel 'slack' requires SLACK_WEBHOOK")
			}
		case "discord":
			if c.DiscordWebhook == "" {
				return fmt.Errorf("channel 'discord' requires DISCORD_WEBHOOK")
			}
		default:
			return fmt.Errorf("unknown notification channel specified: %q", ch)
		}

		cleanChannels = append(cleanChannels, ch)
	}

	if len(seen) == 0 {
		return fmt.Errorf("at least one valid notification channel must be specified")
	}

	c.Channels = cleanChannels
	return nil
}
