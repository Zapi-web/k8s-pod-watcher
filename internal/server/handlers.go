package server

import (
	"net/http"
	"sync/atomic"
)

type HealthHandlers struct {
	isReady atomic.Bool
}

func NewHealthHandlers() *HealthHandlers {
	return &HealthHandlers{}
}

func (h *HealthHandlers) SetStatus(status bool) {
	h.isReady.Store(status)
}

func (h *HealthHandlers) ServeHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

func (h *HealthHandlers) ServeReadyz(w http.ResponseWriter, r *http.Request) {
	if h.isReady.Load() {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write([]byte("UNREADY"))
}
