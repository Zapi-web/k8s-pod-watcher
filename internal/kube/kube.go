package kube

import (
	"fmt"
	"log/slog"
	"path/filepath"

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
