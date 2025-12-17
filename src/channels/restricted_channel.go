package channels

import (
	"fmt"
	"sync"
	"sync/atomic"
)

type RestrictedChannel[T any] struct {
	channel        chan T
	allowedWriters map[int64]struct{}
	allowedReaders map[int64]struct{}
	active         atomic.Bool
	muWriters      sync.RWMutex
	muReaders      sync.RWMutex
}

// ---- constructors ----
func NewBufferedRestrictedChannel[T any](bufferSize int) *RestrictedChannel[T] {
	return &RestrictedChannel[T]{
		channel:        make(chan T, bufferSize),
		allowedWriters: make(map[int64]struct{}),
		allowedReaders: make(map[int64]struct{}),
		active:         atomic.Bool{}, // default is false
	}
}

func NewUnbufferedRestrictedChannel[T any]() *RestrictedChannel[T] {
	return &RestrictedChannel[T]{
		channel:        make(chan T),
		allowedWriters: make(map[int64]struct{}),
		allowedReaders: make(map[int64]struct{}),
		active:         atomic.Bool{},
	}
}

// ---- write/read ----
func (rc *RestrictedChannel[T]) Write(id int64, payload T) error {
	// no read lock required as permission maps are only modified before activation
	if !rc.IsActive() {
		return fmt.Errorf("RestrictedChannel must be activated before writing")
	}
	if _, ok := rc.allowedWriters[id]; !ok {
		return fmt.Errorf("unauthorized write attempt from ID %d", id)
	}
	rc.channel <- payload
	return nil
}

func (rc *RestrictedChannel[T]) Read(id int64) (T, error) {
	// no read lock required as permission maps are only modified before activation
	var zero T
	if !rc.IsActive() {
		return zero, fmt.Errorf("RestrictedChannel must be activated before reading")
	}
	if _, ok := rc.allowedReaders[id]; !ok {
		return zero, fmt.Errorf("unauthorized read attempt from ID %d", id)
	}
	return <-rc.channel, nil
}

// ---- permission management ----
func (rc *RestrictedChannel[T]) AllowWrite(id int64) *RestrictedChannel[T] {
	if rc.IsActive() {
		panic("cannot modify permissions of an active RestrictedChannel")
	}
	rc.muWriters.Lock()
	defer rc.muWriters.Unlock()

	rc.allowedWriters[id] = struct{}{}
	return rc
}

func (rc *RestrictedChannel[T]) AllowRead(id int64) *RestrictedChannel[T] {
	if rc.IsActive() {
		panic("cannot modify permissions of an active RestrictedChannel")
	}
	rc.muReaders.Lock()
	defer rc.muReaders.Unlock()

	rc.allowedReaders[id] = struct{}{}
	return rc
}

func (rc *RestrictedChannel[T]) AllowReadAndWrite(id int64) *RestrictedChannel[T] {
	if rc.IsActive() {
		panic("cannot modify permissions of an active RestrictedChannel")
	}
	rc.muReaders.Lock()
	defer rc.muReaders.Unlock()
	rc.muWriters.Lock()
	defer rc.muWriters.Unlock()

	rc.allowedWriters[id] = struct{}{}
	rc.allowedReaders[id] = struct{}{}
	return rc
}

func (rc *RestrictedChannel[T]) RevokeWrite(id int64) *RestrictedChannel[T] {
	if rc.IsActive() {
		panic("cannot modify permissions of an active RestrictedChannel")
	}
	rc.muWriters.Lock()
	defer rc.muWriters.Unlock()

	delete(rc.allowedWriters, id)
	return rc
}

func (rc *RestrictedChannel[T]) RevokeRead(id int64) *RestrictedChannel[T] {
	if rc.IsActive() {
		panic("cannot modify permissions of an active RestrictedChannel")
	}
	rc.muReaders.Lock()
	defer rc.muReaders.Unlock()

	delete(rc.allowedReaders, id)
	return rc
}

func (rc *RestrictedChannel[T]) RevokeReadAndWrite(id int64) *RestrictedChannel[T] {
	if rc.IsActive() {
		panic("cannot modify permissions of an active RestrictedChannel")
	}
	rc.muWriters.Lock()
	defer rc.muWriters.Unlock()
	rc.muReaders.Lock()
	defer rc.muReaders.Unlock()

	delete(rc.allowedReaders, id)
	delete(rc.allowedWriters, id)
	return rc
}

// ---- activation ----
func (rc *RestrictedChannel[T]) Activate() *RestrictedChannel[T] {
	rc.active.CompareAndSwap(false, true)
	return rc
}

func (rc *RestrictedChannel[T]) IsActive() bool {
	return rc.active.Load()
}
