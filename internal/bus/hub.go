package bus

import (
	"context"
	"sync"
	"time"
)

type Topic string

const (
	TopicSessionStream Topic = "session.stream"
	TopicSessionChat   Topic = "session.chat"
	TopicToolCall      Topic = "tool.call"
	TopicToolResult    Topic = "tool.result"
	TopicSystem        Topic = "system"
)

type Event struct {
	ID        string    `json:"id,omitempty"`
	Topic     Topic     `json:"topic"`
	Workspace string    `json:"workspace"`
	SessionID string    `json:"session_id,omitempty"`
	TaskID    string    `json:"task_id,omitempty"`
	Type      string    `json:"type"` // "delta","done","error","status",...
	Payload   any       `json:"payload,omitempty"`
	Time      time.Time `json:"time"`
}

type Subscription struct {
	ID     string
	Topics map[Topic]bool
	Ch     chan Event
	Close  func()
}

type Hub struct {
	mu   sync.RWMutex
	subs map[string]*Subscription
}

func NewHub() *Hub {
	return &Hub{subs: map[string]*Subscription{}}
}

func (h *Hub) Subscribe(id string, topics ...Topic) *Subscription {
	h.mu.Lock()
	defer h.mu.Unlock()

	tm := map[Topic]bool{}
	for _, t := range topics {
		tm[t] = true
	}

	sub := &Subscription{
		ID:     id,
		Topics: tm,
		Ch:     make(chan Event, 256),
	}
	sub.Close = func() {
		h.mu.Lock()
		if s, ok := h.subs[id]; ok {
			close(s.Ch)
			delete(h.subs, id)
		}
		h.mu.Unlock()
	}
	h.subs[id] = sub
	return sub
}

func (h *Hub) Publish(ev Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, s := range h.subs {
		if !s.Topics[ev.Topic] {
			continue
		}
		select {
		case s.Ch <- ev:
		default:
			// backpressure: drop (7.5에서 per-sub ringbuffer/slow consumer 분리)
		}
	}
}

func (h *Hub) Close(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, s := range h.subs {
		close(s.Ch)
		delete(h.subs, id)
	}
	return nil
}
