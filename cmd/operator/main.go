package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Zapi-web/k8s-pod-watcher/internal/config"
	"github.com/Zapi-web/k8s-pod-watcher/internal/logger"
	"github.com/Zapi-web/k8s-pod-watcher/internal/metrics"
	"github.com/Zapi-web/k8s-pod-watcher/internal/notifier"
	"github.com/Zapi-web/k8s-pod-watcher/internal/watcher"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
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

	client, err := newKubeClient()

	if err != nil {
		slog.Error("failed to get kubernetes clientSet", "err", err)
		return 1
	}

	reg := prometheus.NewRegistry()
	promMetrics := metrics.New(reg)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	srv := &http.Server{
		Addr:    ":" + cfg.MetricsPort,
		Handler: mux,
	}

	go runMetricsServer(ctx, srv, stop)

	watch := watcher.New(client, tgNotif, cfg.ChatID, promMetrics)

	err = watch.Start(ctx)
	if err != nil {
		slog.Error("failed to start kubernetes watcher", "err", err)
		return 1
	}

	slog.Info("system fully started, waiting for Pod failures")
	<-ctx.Done()
	slog.Info("received a signal, starting graceful shutdown")

	slog.Debug("watcher stopped")
	return 0
}

func runMetricsServer(ctx context.Context, srv *http.Server, stop context.CancelFunc) {
	slog.Info("Starting metrics server", "addr", srv.Addr)
	errChan := make(chan error, 1)

	go func(srv *http.Server) {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}(srv)

	select {
	case err := <-errChan:
		if err != nil {
			slog.Error("Metrics server failed", "err", err)
			stop()
		}
	case <-ctx.Done():
		slog.Info("Received a signal. Trying graceful shutdown metrics server")
		servCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := srv.Shutdown(servCtx); err != nil {
			_ = srv.Close()
			slog.Error("Failed to stop metrics server graceful", "err", err)
		}
	}
}

func newKubeClient() (kubernetes.Interface, error) {
	kubeCfg, err := rest.InClusterConfig()

	if err == nil {
		slog.Info("Running inside Kube cluster; using service account")
		return kubernetes.NewForConfig(kubeCfg)
	}

	slog.Info("Failed in-cluster connection; falling back to local kubeconfig.")
	kubeConfigPath := filepath.Join(homedir.HomeDir(), ".kube", "config")
	kubeCfg, err = clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build config from flags: %w", err)
	}

	return kubernetes.NewForConfig(kubeCfg)
}
