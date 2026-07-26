package kube

import (
	"context"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes/fake"
)

func TestNewLeaderElection_Lifecycle(t *testing.T) {
	client := fake.NewSimpleClientset()

	onStopCalled := make(chan struct{})
	onStartCalled := make(chan struct{})

	cfg := LeaderElectionConfig{
		Client:         client,
		LeaseName:      "test-lock",
		LeaseNamespace: "default",
		Identity:       "test-pod-1",
		OnStart: func(ctx context.Context) {
			close(onStartCalled)
		},
		OnStop: func() {
			close(onStopCalled)
		},
	}

	ctx, cancel := context.WithCancel(t.Context())

	errChan := make(chan error, 1)

	go func() {
		errChan <- RunLeaderElection(ctx, cfg)
	}()

	select {
	case <-onStartCalled:

	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting on OnStartCalled callback")
	}

	cancel()

	select {
	case <-onStopCalled:

	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting on OnStopCalled callback")
	}

	if err := <-errChan; err != nil {
		t.Fatalf("unexpected error during leader election: %v", err)
	}
}
