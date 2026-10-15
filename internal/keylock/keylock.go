// Package keylock serializes work per key with a fixed set of mutexes, so a long-running
// process does not grow one mutex for every key it ever sees.
package keylock

import (
	"hash/fnv"
	"sync"
)

const stripes = 256

// Striped hashes each key onto one of 256 mutexes. Two keys can share one, so a caller must
// never take a second key's lock from the same Striped while it holds one: on a shared mutex
// it would wait for itself.
type Striped struct {
	mu [stripes]sync.Mutex
}

// Lock locks key's mutex and returns its unlock.
func (s *Striped) Lock(key string) (unlock func()) {
	m := &s.mu[stripe(key)]
	m.Lock()
	return m.Unlock
}

func stripe(key string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return h.Sum32() % stripes
}
