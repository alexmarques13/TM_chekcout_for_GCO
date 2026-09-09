package links

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadWithHeader(t *testing.T) {
	path := writeTemp(t, "link\nhttps://encsoft.app/api/bot/getLink?id=AAA\nhttps://encsoft.app/api/bot/getLink?id=BBB\n")
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d links, want 2", len(got))
	}
	if got[0] != "https://encsoft.app/api/bot/getLink?id=AAA" {
		t.Fatalf("got %q", got[0])
	}
}

func TestLoadPlainURLs(t *testing.T) {
	path := writeTemp(t, "https://encsoft.app/api/bot/getLink?id=AAA\nhttps://encsoft.app/api/bot/getLink?id=BBB\n")
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d links, want 2", len(got))
	}
}

func writeTemp(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "links.csv")
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}
