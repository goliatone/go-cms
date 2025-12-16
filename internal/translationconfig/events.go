package translationconfig

import (
	"context"
	"sync"
)

type changeBroadcaster struct {
	mu       sync.Mutex
	watchers map[uint64]chan ChangeEvent
	nextID   uint64
}

func newChangeBroadcaster() *changeBroadcaster {
	return &changeBroadcaster{
		watchers: make(map[uint64]chan ChangeEvent),
	}
}

func (b *changeBroadcaster) Subscribe(ctx context.Context) (<-chan ChangeEvent, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		ch := make(chan ChangeEvent)
		close(ch)
		return ch, nil
	}
	ch := make(chan ChangeEvent, 1)

	b.mu.Lock()
	id := b.nextID
	b.nextID++
	b.watchers[id] = ch
	b.mu.Unlock()

	go func() {
		<-ctx.Done()
		b.mu.Lock()
		delete(b.watchers, id)
		close(ch)
		b.mu.Unlock()
	}()

	return ch, nil
}

func (b *changeBroadcaster) Broadcast(evt ChangeEvent) {
	b.mu.Lock()
	watchers := make([]chan ChangeEvent, 0, len(b.watchers))
	for _, ch := range b.watchers {
		watchers = append(watchers, ch)
	}
	b.mu.Unlock()

	for _, ch := range watchers {
		select {
		case ch <- evt:
		default:
		}
	}
}
