package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Zapi-web/k8s-pod-watcher/internal/config"
	"github.com/Zapi-web/k8s-pod-watcher/internal/kube"
	"github.com/Zapi-web/k8s-pod-watcher/internal/logger"
	"github.com/Zapi-web/k8s-pod-watcher/internal/metrics"
	"github.com/Zapi-web/k8s-pod-watcher/internal/notifier"
	"github.com/Zapi-web/k8s-pod-watcher/internal/server"
	"github.com/Zapi-web/k8s-pod-watcher/internal/watcher"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	os.Exit(run())
}

func run() int {
	cfg, err := config.Init()

	if err != nil {
		slog.Error("failed to read configs", "err", err)
		return 1
	}

	slog.SetDefault(logger.New(cfg.LogLevel))
	slog.Info("logger initialized", "lvl", cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	tgNotif, err := notifier.New(cfg.Token)

	if err != nil {
		slog.Error("failed to create notifier", "err", err)
		return 1
	}

	go tgNotif.Start(ctx)
	defer func() {
		slog.Info("trying to stop telegram graceful")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := tgNotif.Stop(ctx); err != nil {
			slog.Error("failed to stop telegram notifier", "err", err)
		}
	}()

	client, err := kube.NewKubeClient()

	if err != nil {
		slog.Error("failed to get kubernetes clientSet", "err", err)
		return 1
	}

	reg := prometheus.NewRegistry()
	promMetrics := metrics.New(reg)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	srv := server.New(cfg.MetricsPort, mux)
	srvErrChan := srv.RunMetricsServer(ctx)

	watch := watcher.New(client, tgNotif, cfg.ChatID, promMetrics)

	err = watch.Start(ctx)
	if err != nil {
		slog.Error("failed to start kubernetes watcher", "err", err)
		return 1
	}
	defer watch.Stop()

	slog.Info("system fully started, waiting for Pod failures")
	select {
	case err = <-srvErrChan:
		if err != nil {
			slog.Error("received an error from metrics server", "err", err)
			stop()
			return 1
		}
	case <-ctx.Done():
		slog.Info("received a signal, starting graceful shutdown")
	}

	slog.Debug("watcher stopped")
	return 0
}
