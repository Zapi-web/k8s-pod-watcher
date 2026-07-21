package watcher

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Zapi-web/k8s-pod-watcher/internal/metrics"
	"github.com/Zapi-web/k8s-pod-watcher/internal/notifier"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type podUpdate struct {
	NewPod *v1.Pod
	OldPod *v1.Pod
}

type PodWatcher struct {
	clientset kubernetes.Interface
	notifier  notifier.Notifier
	chatID    string
	metrics   *metrics.Metrics
	queue     workqueue.TypedRateLimitingInterface[podUpdate]
	wg        sync.WaitGroup
}

func New(clientset kubernetes.Interface, n notifier.Notifier, chatID string, m *metrics.Metrics) *PodWatcher {
	return &PodWatcher{
		clientset: clientset,
		notifier:  n,
		chatID:    chatID,
		metrics:   m,
		queue: workqueue.NewTypedRateLimitingQueue(
			workqueue.NewTypedItemExponentialFailureRateLimiter[podUpdate](
				5*time.Second,
				5*time.Minute,
			),
		),
	}
}

func (p *PodWatcher) Start(ctx context.Context) error {
	slog.Info("Initializing Kubernetes pod informer...")
	factory := informers.NewSharedInformerFactory(p.clientset, 0)
	podInformer := factory.Core().V1().Pods().Informer()

	_, err := podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			p.metrics.IncEvents()
			oldPod, ok1 := oldObj.(*v1.Pod)
			newPod, ok2 := newObj.(*v1.Pod)
			if !ok1 || !ok2 || oldPod == nil || newPod == nil {
				return
			}

			p.queue.Add(podUpdate{
				OldPod: oldPod,
				NewPod: newPod,
			})
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

	for i := 0; i < 5; i++ {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			p.runWorker(ctx)
		}()
	}

	return nil
}

func (p *PodWatcher) Stop() {
	slog.Info("received a signal, shutting down queue...")
	p.queue.ShutDown()
	p.wg.Wait()
	slog.Info("all workers drained successfully")
}

func (p *PodWatcher) runWorker(ctx context.Context) {
	for {
		item, shutdown := p.queue.Get()

		if shutdown {
			return
		}

		err := p.processPodUpdate(ctx, item)

		if err != nil {
			slog.Warn("failed to process update", "err", err)
			p.queue.AddRateLimited(item)
		} else {
			p.queue.Forget(item)
		}
	}
}

func (p *PodWatcher) processPodUpdate(ctx context.Context, item podUpdate) error {
	defer p.queue.Done(item)

	newPod, oldPod := item.NewPod, item.OldPod

	slog.Debug("processing pod update event", "pod", newPod.Name, "namespace", newPod.Namespace)

	for _, newStatus := range newPod.Status.ContainerStatuses {
		var oldStatus v1.ContainerStatus
		foundOld := false

		for _, oldContainerStatus := range oldPod.Status.ContainerStatuses {
			if oldContainerStatus.Name == newStatus.Name {
				oldStatus = oldContainerStatus
				foundOld = true
				break
			}
		}

		isNewWaitingCrash := newStatus.State.Waiting != nil && newStatus.State.Waiting.Reason == "CrashLoopBackOff"
		isOldWaitingCrash := foundOld && oldStatus.State.Waiting != nil && oldStatus.State.Waiting.Reason == "CrashLoopBackOff"

		isNewOOMKilled := newStatus.State.Terminated != nil && newStatus.State.Terminated.Reason == "OOMKilled"
		isOldOOMKilled := foundOld && oldStatus.State.Terminated != nil && oldStatus.State.Terminated.Reason == "OOMKilled"

		isNewPrevOOMKilled := newStatus.LastTerminationState.Terminated != nil && newStatus.LastTerminationState.Terminated.Reason == "OOMKilled"
		isOldPrevOOMKilled := foundOld && oldStatus.LastTerminationState.Terminated != nil && oldStatus.LastTerminationState.Terminated.Reason == "OOMKilled"

		if isNewWaitingCrash && (!foundOld || !isOldWaitingCrash) {
			if err := p.sendAlert(ctx, newPod.Name, newStatus.Name, "CrashLoopBackOff", newStatus.RestartCount); err != nil {
				return err
			}
			continue
		}

		if isNewOOMKilled && (!foundOld || !isOldOOMKilled) {
			if err := p.sendAlert(ctx, newPod.Name, newStatus.Name, "OOMKilled", newStatus.RestartCount); err != nil {
				return err
			}
			continue
		}

		if isNewPrevOOMKilled && (!foundOld || !isOldPrevOOMKilled) {
			if err := p.sendAlert(ctx, newPod.Name, newStatus.Name, "OOMKilled (previous run)", newStatus.RestartCount); err != nil {
				return err
			}
			continue
		}
	}

	return nil
}

func (p *PodWatcher) sendAlert(ctx context.Context, podName, containerName, reason string, restarts int32) error {
	msg := fmt.Sprintf(
		"*Alert: Container Issue Detected!*\n\n"+
			"*Pod*: '%s'\n"+
			"*Container*: '%s'\n"+
			"*Issue*: '%s'\n"+
			"*Restart Count*: '%d'",
		podName, containerName, reason, restarts,
	)

	slog.Warn("identified crash reason; sending an alert", "pod", podName, "reason", reason)
	if err := p.notifier.SendAlert(ctx, p.chatID, msg); err != nil {
		return fmt.Errorf("failed to send an alert: %w", err)
	}
	p.metrics.IncAlerts(reason)
	return nil
}
