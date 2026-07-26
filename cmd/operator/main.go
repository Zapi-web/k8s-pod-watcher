package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
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
	handlers := server.NewHealthHandlers()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", handlers.ServeHealthz)
	mux.HandleFunc("/readyz", handlers.ServeReadyz)

	srv := server.NewServer(cfg.MetricsPort, mux)
	srvErrChan := srv.RunMetricsServer(ctx)

	watch := watcher.New(client, multiNotif, promMetrics)

	electorConf := kube.LeaderElectionConfig{
		Client:         client,
		LeaseName:      cfg.LeaseName,
		LeaseNamespace: cfg.PodNamespace,
		Identity:       cfg.Identity,
		OnStart: func(ctx context.Context) {
			if err := watch.Start(ctx); err != nil {
				slog.Error("failed to start kubernetes watcher", "err", err)
				stop()
			}
			handlers.SetStatus(true)
		},
		OnStop: func() {
			slog.Warn("leadership lost, stopping watcher")
			handlers.SetStatus(false)
			watch.Stop()
		},
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := kube.RunLeaderElection(ctx, electorConf); err != nil {
			slog.Error("leader election error", "err", err)
		}
	}()

	slog.Info("system fully started, waiting for Pod failures")
	select {
	case err = <-srvErrChan:
		if err != nil {
			slog.Error("received an error from metrics server", "err", err)
			stop()
			wg.Wait()
			return 1
		}
	case <-ctx.Done():
		slog.Info("received a signal, starting graceful shutdown")
	}

	<-srvErrChan
	wg.Wait()
	slog.Debug("watcher stopped")
	return 0
}
