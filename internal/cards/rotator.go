package cards

import (
	"fmt"
	"sync"
)

// Rotator hands out cards from top to bottom of the CSV, then wraps to the top.
type Rotator struct {
	mu   sync.Mutex
	next int
}

func (r *Rotator) Next(path string) (Card, int, int, error) {
	if r == nil {
		return Card{}, 0, 0, fmt.Errorf("card rotator is nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	all, err := LoadAll(path)
	if err != nil {
		return Card{}, 0, 0, err
	}
	if len(all) == 0 {
		return Card{}, 0, 0, fmt.Errorf("no cards found in %s", path)
	}

	idx := r.next % len(all)
	r.next = idx + 1
	if r.next >= len(all) {
		r.next = 0
	}
	return all[idx], idx, len(all), nil
}
