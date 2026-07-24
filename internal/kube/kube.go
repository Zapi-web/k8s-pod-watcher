package kube

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func NewKubeClient() (kubernetes.Interface, error) {
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

func GetLastLogs(ctx context.Context, client kubernetes.Interface, namespace, podName, containerName string, tailLines int64) (string, error) {
	opts := &v1.PodLogOptions{
		Container: containerName,
		TailLines: &tailLines,
		Previous:  true,
	}

	req := client.CoreV1().Pods(namespace).GetLogs(podName, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		opts.Previous = false
		req = client.CoreV1().Pods(namespace).GetLogs(podName, opts)
		stream, err = req.Stream(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to open log stream: %w", err)
		}
	}
	defer func() {
		if err := stream.Close(); err != nil {
			slog.Error("failed to close logs stream", "err", err)
		}
	}()

	limitStream := io.LimitReader(stream, 4096)

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, limitStream); err != nil {
		return "", fmt.Errorf("failed to read log stream: %w", err)
	}

	return buf.String(), nil
}
