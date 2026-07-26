package config

import (
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		Name        string
		TestConfig  Config
		ExpectError bool
	}{
		{
			Name: "Clear Config",
			TestConfig: Config{
				Channels:       []string{"telegram", "discord", "slack"},
				Token:          "token",
				ChatID:         "1000",
				MetricsPort:    "8080",
				DiscordWebhook: "WebHook",
				SlackWebhook:   "WebHook",
			},
		},
		{
			Name: "Invalid Non-Numeric Port",
			TestConfig: Config{
				Channels:    []string{"telegram"},
				Token:       "token",
				ChatID:      "1000",
				MetricsPort: "not-port",
			},
			ExpectError: true,
		},
		{
			Name: "Invalid Out-Of-Bounds Port",
			TestConfig: Config{
				Channels:    []string{"telegram"},
				Token:       "token",
				ChatID:      "1000",
				MetricsPort: "99999",
			},
			ExpectError: true,
		},
		{
			Name: "Telegram Missing Chat ID and Token",
			TestConfig: Config{
				Channels:    []string{"telegram"},
				MetricsPort: "8080",
			},
			ExpectError: true,
		},
		{
			Name: "Slack Missing WebHook",
			TestConfig: Config{
				Channels:    []string{"slack"},
				MetricsPort: "8080",
			},
			ExpectError: true,
		},
		{
			Name: "Discord Missing WebHook",
			TestConfig: Config{
				Channels:    []string{"discord"},
				MetricsPort: "8080",
			},
			ExpectError: true,
		},
		{
			Name: "Unknown Channel",
			TestConfig: Config{
				Channels:    []string{"email"},
				MetricsPort: "8080",
			},
			ExpectError: true,
		},
		{
			Name: "Duplicate Channels",
			TestConfig: Config{
				Channels:    []string{"telegram", "telegram"},
				MetricsPort: "8080",
				Token:       "token",
				ChatID:      "1000",
			},
			ExpectError: true,
		},
		{
			Name: "No channels",
			TestConfig: Config{
				MetricsPort: "8080",
			},
			ExpectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			err := tt.TestConfig.Validate()

			if err != nil && !tt.ExpectError {
				t.Fatalf("unexpected error during validation: %v", err)
			}

			if err == nil && tt.ExpectError {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}
