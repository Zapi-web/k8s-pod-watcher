package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

type Server struct {
	srv *http.Server
}

func New(port string, handler http.Handler) *Server {
	return &Server{
		srv: &http.Server{
			Addr:    ":" + port,
			Handler: handler,
		},
	}
}

func (s *Server) RunMetricsServer(ctx context.Context) <-chan error {
	slog.Info("Starting metrics server", "addr", s.srv.Addr)
	errChan := make(chan error, 1)

	go func() {
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	go func() {
		<-ctx.Done()
		slog.Info("received a signal; draining HTTP server connection")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := s.srv.Shutdown(shutdownCtx); err != nil {
			_ = s.srv.Close()
			slog.Error("failed to shutdown HTTP server gracefully", "err", err)
		}
	}()

	return errChan
}
