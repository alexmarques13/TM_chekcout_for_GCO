package discordwatch

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
)

var (
	pendingFile = "pending_carts.json"
	pendingMu   sync.Mutex
)

func InitPending(path string) (int, error) {
	pendingMu.Lock()
	defer pendingMu.Unlock()
	if p := strings.TrimSpace(path); p != "" {
		pendingFile = p
	}
	pendingCarts.Range(func(k, _ any) bool {
		pendingCarts.Delete(k)
		return true
	})
	carts, err := readPendingLocked()
	if err != nil {
		return 0, err
	}
	n := 0
	for _, c := range carts {
		c.ID = strings.TrimSpace(c.ID)
		c.Link = strings.TrimSpace(c.Link)
		if c.ID == "" || c.Link == "" {
			continue
		}
		pendingCarts.Store(c.ID, c)
		n++
	}
	return n, nil
}

func storePending(c Cart) error {
	c.ID = strings.TrimSpace(c.ID)
	c.Link = strings.TrimSpace(c.Link)
	if c.ID == "" || c.Link == "" {
		return nil
	}
	pendingCarts.Store(c.ID, c)
	return writePending()
}

func takePending(id string) (Cart, bool) {
	id = strings.TrimSpace(id)
	raw, ok := pendingCarts.LoadAndDelete(id)
	if !ok {
		return Cart{}, false
	}
	_ = writePending()
	return raw.(Cart), true
}

func writePending() error {
	pendingMu.Lock()
	defer pendingMu.Unlock()
	return writePendingLocked()
}

func writePendingLocked() error {
	carts := make([]Cart, 0)
	pendingCarts.Range(func(_, v any) bool {
		if c, ok := v.(Cart); ok {
			c.ID = strings.TrimSpace(c.ID)
			c.Link = strings.TrimSpace(c.Link)
			if c.ID != "" && c.Link != "" {
				carts = append(carts, c)
			}
		}
		return true
	})
	raw, err := json.MarshalIndent(carts, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(pendingFile, raw, 0644)
}

func readPendingLocked() ([]Cart, error) {
	raw, err := os.ReadFile(pendingFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil, nil
	}
	var carts []Cart
	if err := json.Unmarshal(raw, &carts); err != nil {
		return nil, err
	}
	return carts, nil
}
