package watcher

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Zapi-web/k8s-pod-watcher/internal/notifier"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type PodWatcher struct {
	clientset kubernetes.Interface
	notifier  notifier.Notifier
	chatID    string
}

func New(clientset kubernetes.Interface, n notifier.Notifier, chatID string) *PodWatcher {
	return &PodWatcher{
		clientset: clientset,
		notifier:  n,
		chatID:    chatID,
	}
}

func (p *PodWatcher) Start(ctx context.Context) error {
	slog.Info("Initializing Kubernetes pod informer...")
	factory := informers.NewSharedInformerFactory(p.clientset, 0)
	podInformer := factory.Core().V1().Pods().Informer()

	_, err := podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			pod, ok := newObj.(*v1.Pod)
			if !ok {
				return
			}
			p.checkPod(ctx, pod)
		},
	})

	if err != nil {
		return fmt.Errorf("failed to add event handler: %w", err)
	}

	go podInformer.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), podInformer.HasSynced) {
		return fmt.Errorf("failed to sync informer cache")
	}

	slog.Info("kubernetes pod informer synced successfully")
	return nil
}

func (p *PodWatcher) checkPod(ctx context.Context, pod *v1.Pod) {

}
