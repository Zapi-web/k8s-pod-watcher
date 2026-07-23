package kube

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

type LeaderElectionConfig struct {
	Client         kubernetes.Interface
	LeaseName      string
	LeaseNamespace string
	Identity       string
	OnStart        func(context.Context)
	OnStop         func()
}

func RunLeaderElection(ctx context.Context, cfg LeaderElectionConfig) error {
	if cfg.Identity == "" {
		host, err := os.Hostname()

		if err != nil {
			return fmt.Errorf("failed to get pod identify: %w", err)
		}
		cfg.Identity = host
	}

	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      cfg.LeaseName,
			Namespace: cfg.LeaseNamespace,
		},
		Client: cfg.Client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: cfg.Identity,
		},
	}

	elector, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   60 * time.Second,
		RenewDeadline:   15 * time.Second,
		RetryPeriod:     5 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				slog.Info("became leader, starting watcher", "identity", cfg.Identity)
				cfg.OnStart(ctx)
			},
			OnStoppedLeading: func() {
				slog.Warn("lost leadership, stopping watcher", "identity", cfg.Identity)
				if cfg.OnStop != nil {
					cfg.OnStop()
				}
			},
			OnNewLeader: func(identity string) {
				if identity == cfg.Identity {
					return
				}
				slog.Info("new leader elected", "leader", identity)
			},
		},
	})

	if err != nil {
		return fmt.Errorf("failed to create leader elector: %w", err)
	}

	elector.Run(ctx)
	return nil
}
