package main

import (
	"net/http"
	"sync"
	"time"
)

// requestGate admits handlers until shutdown begins, then tracks the admitted
// handlers to keep runtime state alive until each one has returned.
type requestGate struct {
	mu        sync.Mutex
	accepting bool
	active    int
	drained   chan struct{}
	closed    bool
}

func newRequestGate() *requestGate {
	return &requestGate{accepting: true, drained: make(chan struct{})}
}

func (gate *requestGate) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		gate.mu.Lock()
		if !gate.accepting {
			gate.mu.Unlock()
			writer.Header().Set("Cache-Control", "no-store")
			writer.Header().Set("Content-Type", "application/json; charset=utf-8")
			writer.WriteHeader(http.StatusServiceUnavailable)
			_, _ = writer.Write([]byte(`{"ok":false,"code":"shutting_down"}`))
			return
		}
		gate.active++
		gate.mu.Unlock()

		defer func() {
			gate.mu.Lock()
			gate.active--
			gate.closeWhenDrainedLocked()
			gate.mu.Unlock()
		}()
		next.ServeHTTP(writer, request)
	})
}

func (gate *requestGate) StopAdmission() {
	gate.mu.Lock()
	gate.accepting = false
	gate.closeWhenDrainedLocked()
	gate.mu.Unlock()
}

func (gate *requestGate) Drained() <-chan struct{} {
	return gate.drained
}

func (gate *requestGate) WaitForDrain(deadline time.Time) bool {
	select {
	case <-gate.drained:
		return true
	default:
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return false
	}
	timer := time.NewTimer(remaining)
	defer timer.Stop()
	select {
	case <-gate.drained:
		return true
	case <-timer.C:
		return false
	}
}

func (gate *requestGate) closeWhenDrainedLocked() {
	if !gate.accepting && gate.active == 0 && !gate.closed {
		close(gate.drained)
		gate.closed = true
	}
}
