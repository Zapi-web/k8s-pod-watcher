package main

import (
	"context"
	"fmt"
	"log/slog"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/Zapi-web/k8s-pod-watcher/internal/config"
	"github.com/Zapi-web/k8s-pod-watcher/internal/logger"
	"github.com/Zapi-web/k8s-pod-watcher/internal/notifier"
	"github.com/Zapi-web/k8s-pod-watcher/internal/watcher"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func main() {
	cfg, err := config.Init()

	if err != nil {
		slog.Error("failed to read configs", "err", err)
		return
	}

	slog.SetDefault(logger.New(cfg.LogLevel))
	slog.Info("logger initialized", "lvl", cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	tgNotif, err := notifier.New(cfg.Token)

	if err != nil {
		slog.Error("failed to create notifier", "err", err)
		return
	}

	go tgNotif.Start(ctx)
	defer tgNotif.Stop(context.Background())

	client, err := newKubeClient()

	if err != nil {
		slog.Error("failed to get kubernetes clientSet", "err", err)
		return
	}

	watch := watcher.New(client, tgNotif, cfg.ChatID)

	err = watch.Start(ctx)
	if err != nil {
		slog.Error("failed to start kubernetes watcher", "err", err)
		return
	}

	slog.Info("system fully started, waiting for Pod failures")
	<-ctx.Done()
	slog.Info("received a signal, starting graceful shutdown")

	slog.Debug("watcher stopped")
}

func newKubeClient() (kubernetes.Interface, error) {
	config, err := rest.InClusterConfig()

	if err == nil {
		slog.Info("Running inside Kube cluster; using service account")
		return kubernetes.NewForConfig(config)
	}

	slog.Info("Failed in-cluster connection; falling back to local kubeconfig.")
	kubeConfigPath := filepath.Join(homedir.HomeDir(), ".kube", "config")
	config, err = clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build config from flags: %w", err)
	}

	return kubernetes.NewForConfig(config)
}
