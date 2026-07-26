package notifier

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

type mockNotifier struct {
	err error
}

func (m *mockNotifier) SendAlert(ctx context.Context, msg string) error {
	return m.err
}

func TestMultiNotifier_SendAlert(t *testing.T) {
	tests := []struct {
		Name        string
		Notifiers   []Notifier
		expectError bool
	}{
		{
			Name: "All Notifiers Succeed",
			Notifiers: []Notifier{
				&mockNotifier{err: nil},
				&mockNotifier{err: nil},
				&mockNotifier{err: nil},
			},
			expectError: false,
		},
		{
			Name: "Partial Notifier Failure",
			Notifiers: []Notifier{
				&mockNotifier{err: nil},
				&mockNotifier{err: errors.New("slack webhook error")},
				&mockNotifier{err: nil},
			},
			expectError: true,
		},
		{
			Name: "All Notifiers Fail",
			Notifiers: []Notifier{
				&mockNotifier{err: errors.New("telegram API error")},
				&mockNotifier{err: errors.New("slack webhook error")},
				&mockNotifier{err: errors.New("discord webhook error")},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			multi := newMulti(tt.Notifiers...)
			err := multi.SendAlert(t.Context(), "test alert")

			if err != nil && !tt.expectError {
				t.Fatalf("unexpected error during dispatch: %v", err)
			}

			if err == nil && tt.expectError {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestInitMulti_NoValidChannels(t *testing.T) {
	cfg := &NotifierDependencies{}
	_, err := InitMulti(cfg, []string{"email"})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestNotifiers_SendAlert(t *testing.T) {
	tests := []struct {
		Name         string
		Setup        func(baseURL string) Notifier
		ExpectedJSON string
	}{
		{
			Name: "Discord Payload & EndPoint",
			Setup: func(baseURL string) Notifier {
				return newDiscord(nil, baseURL)
			},
			ExpectedJSON: `{"content":"Alert!"}`,
		},
		{
			Name: "Slack Payload & EndPoint",
			Setup: func(baseURL string) Notifier {
				return newSlack(nil, baseURL)
			},
			ExpectedJSON: `{"text":"Alert!"}`,
		},
		{
			Name: "Telegram Payload & Custom EndPoint",
			Setup: func(baseURL string) Notifier {
				return newTelegram("token", "12345", baseURL, nil)
			},
			ExpectedJSON: `{"chat_id":"12345","parse_mode":"Markdown","text":"Alert!"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("expected HTTP method %s, got %s", http.MethodPost, r.Method)
				}

				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("failed to read request body: %v", err)
				}

				if !jsonEqual(t, string(body), tt.ExpectedJSON) {
					t.Errorf("JSON payload mismatch:\n got: %s\n want: %s", string(body), tt.ExpectedJSON)
				}

				w.WriteHeader(http.StatusOK)
			}))
			defer mockServer.Close()

			notifier := tt.Setup(mockServer.URL)

			err := notifier.SendAlert(t.Context(), "Alert!")
			if err != nil {
				t.Fatalf("unexpected error sending alert: %v", err)
			}
		})
	}
}

func jsonEqual(t *testing.T, a, b string) bool {
	t.Helper()
	var j1, j2 any

	if err := json.Unmarshal([]byte(a), &j1); err != nil {
		t.Fatalf("failed to unmarshal actual JSON: %v", err)
		return false
	}
	if err := json.Unmarshal([]byte(b), &j2); err != nil {
		t.Fatalf("failed to unmarshal expected JSON: %v", err)
		return false
	}

	return reflect.DeepEqual(j1, j2)
}
