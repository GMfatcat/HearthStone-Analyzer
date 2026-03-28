package jobs

import "sync"

type ExecutionGate struct {
	mu      sync.Mutex
	running map[string]bool
}

func NewExecutionGate() *ExecutionGate {
	return &ExecutionGate{
		running: map[string]bool{},
	}
}

func (g *ExecutionGate) TryAcquire(key string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.running[key] {
		return false
	}

	g.running[key] = true
	return true
}

func (g *ExecutionGate) Release(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	delete(g.running, key)
}
