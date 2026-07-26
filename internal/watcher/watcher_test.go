package watcher

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Zapi-web/k8s-pod-watcher/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/util/workqueue"
)

type mockNotifier struct {
	sendMessages []string
}

func (m *mockNotifier) SendAlert(ctx context.Context, reason string) error {
	m.sendMessages = append(m.sendMessages, reason)
	return nil
}

func TestProcessPodUpdate(t *testing.T) {
	tests := []struct {
		name           string
		oldPod         *v1.Pod
		newPod         *v1.Pod
		expectAlert    bool
		expectedIssues []string
	}{
		{
			name: "Update to CrashLoopBackOff",
			oldPod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "app-pod", Namespace: "default"},
				Status: v1.PodStatus{
					ContainerStatuses: []v1.ContainerStatus{
						{Name: "app", State: v1.ContainerState{Running: &v1.ContainerStateRunning{}}},
					},
				},
			},
			newPod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "app-pod", Namespace: "default"},
				Status: v1.PodStatus{
					ContainerStatuses: []v1.ContainerStatus{
						{
							Name: "app", State: v1.ContainerState{
								Waiting: &v1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
							},
							RestartCount: 3,
						},
					},
				},
			},
			expectAlert:    true,
			expectedIssues: []string{"CrashLoopBackOff"},
		},
		{
			name: "Staying in CrashLoopBackOff should not spam",
			oldPod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "app-pod", Namespace: "default"},
				Status: v1.PodStatus{
					ContainerStatuses: []v1.ContainerStatus{
						{
							Name: "app", State: v1.ContainerState{
								Waiting: &v1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
							},
						},
					},
				},
			},
			newPod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "app-pod", Namespace: "default"},
				Status: v1.PodStatus{
					ContainerStatuses: []v1.ContainerStatus{
						{
							Name: "app", State: v1.ContainerState{
								Waiting: &v1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
							},
						},
					},
				},
			},
			expectAlert: false,
		},
		{
			name: "Update to OOMKilled",
			oldPod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "app-pod", Namespace: "default"},
				Status: v1.PodStatus{
					ContainerStatuses: []v1.ContainerStatus{
						{Name: "app", State: v1.ContainerState{Running: &v1.ContainerStateRunning{}}},
					},
				},
			},
			newPod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "app-pod", Namespace: "default"},
				Status: v1.PodStatus{
					ContainerStatuses: []v1.ContainerStatus{
						{
							Name: "app", State: v1.ContainerState{
								Terminated: &v1.ContainerStateTerminated{Reason: "OOMKilled"},
							},
							RestartCount: 3,
						},
					},
				},
			},
			expectAlert:    true,
			expectedIssues: []string{"OOMKilled"},
		},
		{
			name: "Update to OOMKilled (previous run)",
			oldPod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "app-pod", Namespace: "default"},
				Status: v1.PodStatus{
					ContainerStatuses: []v1.ContainerStatus{
						{Name: "app", State: v1.ContainerState{Running: &v1.ContainerStateRunning{}}},
					},
				},
			},
			newPod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "app-pod", Namespace: "default"},
				Status: v1.PodStatus{
					ContainerStatuses: []v1.ContainerStatus{
						{
							Name: "app", LastTerminationState: v1.ContainerState{
								Terminated: &v1.ContainerStateTerminated{Reason: "OOMKilled"},
							},
							RestartCount: 3,
						},
					},
				},
			},
			expectAlert:    true,
			expectedIssues: []string{"OOMKilled (previous run)"},
		},
		{
			name: "Multi-container failure",
			oldPod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "multi-pod", Namespace: "default"},
				Status: v1.PodStatus{
					ContainerStatuses: []v1.ContainerStatus{
						{Name: "app1", State: v1.ContainerState{Running: &v1.ContainerStateRunning{}}},
						{Name: "app2", State: v1.ContainerState{Running: &v1.ContainerStateRunning{}}},
					},
				},
			},
			newPod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "multi-pod", Namespace: "default"},
				Status: v1.PodStatus{
					ContainerStatuses: []v1.ContainerStatus{
						{Name: "app1", State: v1.ContainerState{Waiting: &v1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}}},
						{Name: "app2", State: v1.ContainerState{Waiting: &v1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}}},
					},
				},
			},
			expectAlert:    true,
			expectedIssues: []string{"CrashLoopBackOff", "CrashLoopBackOff"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewSimpleClientset()
			mockNot := &mockNotifier{}

			reg := prometheus.NewRegistry()
			promMetrics := metrics.New(reg)

			pw := New(fakeClient, mockNot, promMetrics)
			pw.queue = workqueue.NewTypedRateLimitingQueue(
				workqueue.NewTypedItemExponentialFailureRateLimiter[podUpdate](5*time.Second, 5*time.Minute),
			)

			err := pw.processPodUpdate(t.Context(), podUpdate{
				NewPod: tt.newPod,
				OldPod: tt.oldPod,
			})

			if err != nil && tt.expectAlert == false {
				t.Fatalf("unexpected error during processing: %v", err)
			}

			if tt.expectAlert {
				if len(mockNot.sendMessages) != len(tt.expectedIssues) {
					t.Fatalf("expected %d alert, got %d", len(tt.expectedIssues), len(mockNot.sendMessages))
				}

				for i, expect := range tt.expectedIssues {
					if !strings.Contains(mockNot.sendMessages[i], expect) {
						t.Fatalf("at index %d:expected alert to contain %q, got %q", i, expect, mockNot.sendMessages[i])
					}
				}
			} else {
				if len(mockNot.sendMessages) != 0 {
					t.Fatalf("expected 0 alerts, got %d", len(mockNot.sendMessages))
				}
			}
		})
	}
}

func TestPodWatcher_Lifecycle_StartAndStop(t *testing.T) {
	client := fake.NewClientset()
	reg := prometheus.NewRegistry()
	testWatcher := New(client, &mockNotifier{}, metrics.New(reg))

	ctx1, cancel1 := context.WithCancel(t.Context())
	err := testWatcher.Start(ctx1)

	if err != nil {
		t.Fatalf("unexpected error at first launch, got %v", err)
	}

	cancel1()
	testWatcher.Stop()

	ctx2, cancel2 := context.WithCancel(t.Context())

	err = testWatcher.Start(ctx2)

	if err != nil {
		t.Fatalf("unexpected error at second launch, got %v", err)
	}
	cancel2()
	testWatcher.Stop()
}
