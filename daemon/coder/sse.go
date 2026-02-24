package coder

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

// SSEHub broadcasts events to all connected SSE clients.
type SSEHub struct {
	mu      sync.RWMutex
	clients map[chan []byte]struct{}
}

func newSSEHub() *SSEHub {
	return &SSEHub{
		clients: make(map[chan []byte]struct{}),
	}
}

func (h *SSEHub) subscribe() (chan []byte, func()) {
	ch := make(chan []byte, 64)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.clients, ch)
		h.mu.Unlock()
		close(ch)
	}
}

// Broadcast sends a named SSE event to all subscribers.
func (h *SSEHub) Broadcast(eventName string, data any) {
	payload, err := json.Marshal(data)
	if err != nil {
		log.Printf("coder sse: marshal %s: %v", eventName, err)
		return
	}
	msg := append([]byte("event: "+eventName+"\ndata: "), payload...)
	msg = append(msg, '\n', '\n')

	h.mu.RLock()
	for ch := range h.clients {
		select {
		case ch <- msg:
		default:
			// slow client — drop
		}
	}
	h.mu.RUnlock()
}

// ServeSSE streams events to the HTTP response. Call with a long write deadline.
func (h *SSEHub) ServeSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch, unsub := h.subscribe()
	defer unsub()

	// keepalive ticker
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			_, _ = w.Write(msg)
			flusher.Flush()
		case <-ticker.C:
			_, _ = w.Write([]byte(": keepalive\n\n"))
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
