package shared

import (
	"context"
	"sync"
	"time"
)

type Pending struct {
	mu      sync.Mutex
	waiters map[string]chan HttpResponseMessage
}

func NewPending() *Pending {
	return &Pending{waiters: make(map[string]chan HttpResponseMessage)}
}

func (pending *Pending) Wait(id string, ctx context.Context, timeout time.Duration) (HttpResponseMessage, error) {
	waiter := make(chan HttpResponseMessage, 1)
	pending.mu.Lock()
	pending.waiters[id] = waiter
	pending.mu.Unlock()
	defer pending.Cancel(id)

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case message := <-waiter:
		return message, nil
	case <-ctx.Done():
		return HttpResponseMessage{}, ctx.Err()
	case <-timer.C:
		return HttpResponseMessage{}, context.DeadlineExceeded
	}
}

func (pending *Pending) Deliver(message HttpResponseMessage) {
	pending.mu.Lock()
	waiter := pending.waiters[message.RequestID]
	pending.mu.Unlock()
	if waiter == nil {
		return
	}
	select {
	case waiter <- message:
	default:
	}
}

func (pending *Pending) Cancel(id string) {
	pending.mu.Lock()
	delete(pending.waiters, id)
	pending.mu.Unlock()
}
