package watcher

import (
	"context"
	"strings"
	"testing"

	"github.com/Zapi-web/k8s-pod-watcher/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

type mockNotifier struct {
	sendMessages []string
}

func (m *mockNotifier) SendAlert(ctx context.Context, chatID string, reason string) error {
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

			pw := New(fakeClient, mockNot, "test-chat-id", promMetrics)

			pw.processPodUpdate(t.Context(), tt.oldPod, tt.newPod)

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
