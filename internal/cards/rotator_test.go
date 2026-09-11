package cards

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"
)

func TestRotatorWrapsFromTop(t *testing.T) {
	path := writeTempCSV(t, [][]string{
		{"card_number", "exp", "cvv", "name", "address", "city", "state", "zip", "phone"},
		{"4111111111111111", "12/29", "123", "A", "1 St", "Columbus", "OH", "43215", "111"},
		{"4000056655665556", "06/30", "456", "B", "2 St", "Atlanta", "GA", "30303", "222"},
		{"5555555555554444", "01/28", "789", "C", "3 St", "Dallas", "TX", "75001", "333"},
	})
	var r Rotator
	want := []string{"1111", "5556", "4444", "1111", "5556"}
	for i, last4 := range want {
		card, idx, total, err := r.Next(path)
		if err != nil {
			t.Fatal(err)
		}
		if total != 3 {
			t.Fatalf("total=%d", total)
		}
		if card.Last4() != last4 {
			t.Fatalf("step %d: got %s want %s idx=%d", i, card.Last4(), last4, idx)
		}
	}
}

func TestRotatorSingleCard(t *testing.T) {
	path := writeTempCSV(t, [][]string{
		{"card_number", "exp", "cvv", "name", "address", "city", "state", "zip", "phone"},
		{"4111111111111111", "12/29", "123", "A", "1 St", "Columbus", "OH", "43215", "111"},
	})
	var r Rotator
	for i := 0; i < 3; i++ {
		card, idx, total, err := r.Next(path)
		if err != nil {
			t.Fatal(err)
		}
		if idx != 0 || total != 1 || card.Last4() != "1111" {
			t.Fatalf("i=%d idx=%d total=%d last4=%s", i, idx, total, card.Last4())
		}
	}
}

func writeTempCSV(t *testing.T, rows [][]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cards.csv")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := csv.NewWriter(f).WriteAll(rows); err != nil {
		t.Fatal(err)
	}
	return path
}
