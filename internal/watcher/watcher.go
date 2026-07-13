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
			oldPod, ok1 := oldObj.(*v1.Pod)
			newPod, ok2 := newObj.(*v1.Pod)
			if !ok1 || !ok2 || oldPod == nil || newPod == nil {
				return
			}
			p.processPodUpdate(ctx, oldPod, newPod)
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

func (p *PodWatcher) processPodUpdate(ctx context.Context, oldPod, newPod *v1.Pod) {
	for _, newStatus := range newPod.Status.ContainerStatuses {
		var oldStatus v1.ContainerStatus
		foundOld := false

		for _, os := range oldPod.Status.ContainerStatuses {
			if os.Name == newStatus.Name {
				oldStatus = os
				foundOld = true
				break
			}
		}

		if foundOld && newStatus.RestartCount > oldStatus.RestartCount {
			if newStatus.State.Waiting != nil && newStatus.State.Waiting.Reason == "CrashLoopBackOff" {
				p.sendAlert(ctx, newPod.Name, newStatus.Name, "CrashLoopBackOff", newStatus.RestartCount)
				return
			}

			if newStatus.State.Terminated != nil && newStatus.State.Terminated.Reason == "OOMKilled" {
				p.sendAlert(ctx, newPod.Name, newStatus.Name, "OOMKilled", newStatus.RestartCount)
				return
			}

			if newStatus.LastTerminationState.Terminated != nil && newStatus.LastTerminationState.Terminated.Reason == "OOMKilled" {
				p.sendAlert(ctx, newPod.Name, newStatus.Name, "OOMKilled (Previous run)", newStatus.RestartCount)
				return
			}
		}
	}
}

func (p *PodWatcher) sendAlert(ctx context.Context, podName, containerName, reason string, restarts int32) {
	msg := fmt.Sprintf(
		"*Alert: Container Issue Detected!*\n\n"+
			"*Pod*: '%s'\n"+
			"*Container*: '%s'\n"+
			"*Issue*: '%s'\n"+
			"*Restart Count*: '%d'",
		podName, containerName, reason, restarts,
	)

	slog.Warn("indentified crash reason; sending an alert", "pod", podName, "reason", reason)
	if err := p.notifier.SendAlert(ctx, p.chatID, msg); err != nil {
		slog.Error("failed to send a alert", "err", err)
	}
}
