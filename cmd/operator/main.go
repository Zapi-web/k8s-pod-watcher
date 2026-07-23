package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

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

	multiNotif, err := notifier.InitMulti(&notifier.NotifierDependencies{
		TgToken:        cfg.Token,
		TgChatID:       cfg.ChatID,
		SlackWebHook:   cfg.SlackWebhook,
		DiscordWebHook: cfg.DiscordWebhook,
	}, cfg.Channels)

	if err != nil {
		slog.Error("failed to initialize notifiers", "err", err)
		return 1
	}

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

	watch := watcher.New(client, multiNotif, promMetrics)

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
