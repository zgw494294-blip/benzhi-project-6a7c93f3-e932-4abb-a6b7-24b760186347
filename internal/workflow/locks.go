package workflow

import "sync"

type caseLocks struct {
	mu    sync.Mutex
	locks map[string]*lockRef
}

type lockRef struct {
	mu   sync.Mutex
	refs int
}

func newCaseLocks() *caseLocks { return &caseLocks{locks: map[string]*lockRef{}} }

func (c *caseLocks) lock(key string) func() {
	c.mu.Lock()
	ref := c.locks[key]
	if ref == nil {
		ref = &lockRef{}
		c.locks[key] = ref
	}
	ref.refs++
	c.mu.Unlock()
	ref.mu.Lock()
	return func() {
		ref.mu.Unlock()
		c.mu.Lock()
		ref.refs--
		if ref.refs == 0 {
			delete(c.locks, key)
		}
		c.mu.Unlock()
	}
}
