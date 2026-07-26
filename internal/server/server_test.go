package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealthHandlers_ServeHealthz(t *testing.T) {
	handlers := NewHealthHandlers()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	handlers.ServeHealthz(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestHealthHandlers_ServeReadyz(t *testing.T) {
	tests := []struct {
		Name           string
		InitialReady   bool
		ExpectedStatus int
	}{
		{
			Name:           "Unready State Returns 503",
			InitialReady:   false,
			ExpectedStatus: http.StatusServiceUnavailable,
		},
		{
			Name:           "Ready State Returns 200",
			InitialReady:   true,
			ExpectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			handlers := NewHealthHandlers()
			handlers.SetStatus(tt.InitialReady)

			req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
			rec := httptest.NewRecorder()

			handlers.ServeReadyz(rec, req)

			if rec.Code != tt.ExpectedStatus {
				t.Fatalf("expected status %d, got %d", tt.ExpectedStatus, rec.Code)
			}
		})
	}
}

func TestRunMetricsServer_LifecycleAndShutdown(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to allocate free port: %v", err)
	}
	port := fmt.Sprintf("%d", listener.Addr().(*net.TCPAddr).Port)
	_ = listener.Close()

	handlers := NewHealthHandlers()
	handlers.SetStatus(true)

	mux := http.NewServeMux()
	mux.HandleFunc("/readyz", handlers.ServeReadyz)
	mux.HandleFunc("/healthz", handlers.ServeHealthz)

	srv := NewServer(port, mux)

	ctx, cancel := context.WithCancel(t.Context())
	errChan := srv.RunMetricsServer(ctx)

	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%s/readyz", port))
	if err != nil {
		t.Fatalf("failed to send request on running server: %v", err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	resp, err = http.Get(fmt.Sprintf("http://127.0.0.1:%s/healthz", port))
	if err != nil {
		t.Fatalf("failed to send request on running server: %v", err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	cancel()

	select {
	case err, ok := <-errChan:
		if ok && err != nil {
			t.Fatalf("unexpected error during graceful shutdown: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for server shutdown")
	}
}
