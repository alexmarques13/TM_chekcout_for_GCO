package links

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"
)

func Load(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open links csv: %w", err)
	}
	defer f.Close()

	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read links csv: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no links found in %s", path)
	}

	start := 0
	if looksLikeHeader(rows[0]) {
		start = 1
	}

	var out []string
	for _, values := range rows[start:] {
		if len(values) == 0 {
			continue
		}
		url := strings.TrimSpace(values[0])
		url = strings.TrimPrefix(url, "\ufeff")
		if url == "" || strings.HasPrefix(url, "#") {
			continue
		}
		out = append(out, url)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no getLink URLs found in %s", path)
	}
	return out, nil
}

func looksLikeHeader(row []string) bool {
	if len(row) == 0 {
		return false
	}
	first := strings.ToLower(strings.TrimSpace(row[0]))
	first = strings.TrimPrefix(first, "\ufeff")
	switch first {
	case "link", "url", "getlink", "get_link":
		return true
	}
	return !strings.HasPrefix(first, "http://") && !strings.HasPrefix(first, "https://")
}
