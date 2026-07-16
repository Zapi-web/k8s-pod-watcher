package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	totalAlerts        *prometheus.CounterVec
	podEventsProcessed prometheus.Counter
}

func New(reg prometheus.Registerer) *Metrics {
	return &Metrics{
		totalAlerts: promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
			Name: "watcher_total_alerts",
			Help: "Total alerts from watcher",
		}, []string{"reason"}),

		podEventsProcessed: promauto.With(reg).NewCounter(prometheus.CounterOpts{
			Name: "watcher_total_events",
			Help: "Total events processed from informer",
		}),
	}

}

func (m *Metrics) IncAlerts(reason string) {
	m.totalAlerts.WithLabelValues(reason).Inc()
}

func (m *Metrics) IncEvents() {
	m.podEventsProcessed.Inc()
}
