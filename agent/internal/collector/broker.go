package collector

import "sync"

type Broker struct {
	mu       sync.RWMutex
	nextID   uint64
	capacity int
	subs     map[uint64]chan Event
}

func NewBroker(capacity int) *Broker {
	if capacity <= 0 {
		capacity = 256
	}
	return &Broker{capacity: capacity, subs: make(map[uint64]chan Event)}
}

func (b *Broker) Subscribe() (<-chan Event, func()) {
	b.mu.Lock()
	b.nextID++
	id := b.nextID
	ch := make(chan Event, b.capacity)
	b.subs[id] = ch
	b.mu.Unlock()
	var once sync.Once
	return ch, func() {
		once.Do(func() {
			b.mu.Lock()
			delete(b.subs, id)
			close(ch)
			b.mu.Unlock()
		})
	}
}

func (b *Broker) Publish(event Event) uint64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	var dropped uint64
	for _, subscriber := range b.subs {
		select {
		case subscriber <- event:
		default:
			dropped++
		}
	}
	return dropped
}
