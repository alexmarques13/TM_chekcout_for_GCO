package discordwatch

import (
	"path/filepath"
	"testing"
)

func TestPendingSaveReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pending_carts.json")
	if n, err := InitPending(path); err != nil || n != 0 {
		t.Fatalf("init empty: n=%d err=%v", n, err)
	}
	if err := storePending(Cart{ID: "111", Link: "https://encsoft.app/api/bot/getLink?id=EC4B136B4D7A"}); err != nil {
		t.Fatal(err)
	}
	if err := storePending(Cart{ID: "222", Link: "https://encsoft.app/api/bot/getLink?id=ABC"}); err != nil {
		t.Fatal(err)
	}

	n, err := InitPending(path)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("loaded=%d", n)
	}
	cart, ok := takePending("111")
	if !ok || cart.Link != "https://encsoft.app/api/bot/getLink?id=EC4B136B4D7A" {
		t.Fatalf("take 111: ok=%v cart=%+v", ok, cart)
	}

	n, err = InitPending(path)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("after take loaded=%d", n)
	}
	if _, ok := takePending("111"); ok {
		t.Fatal("111 should be gone")
	}
	cart, ok = takePending("222")
	if !ok || cart.ID != "222" {
		t.Fatalf("take 222: ok=%v cart=%+v", ok, cart)
	}
	if _, ok := takePending("222"); ok {
		t.Fatal("222 should be gone")
	}
}
