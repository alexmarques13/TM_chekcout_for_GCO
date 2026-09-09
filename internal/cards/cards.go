package cards

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"
)

type Card struct {
	Number   string
	ExpMonth string
	ExpYear  string
	CVV      string
	Name     string
	Address  string
	City     string
	State    string
	Zip      string
	Phone    string
	Email    string
	Account  string
}

func (c Card) Last4() string {
	n := c.Number
	if len(n) >= 4 {
		return n[len(n)-4:]
	}
	return n
}

func (c Card) ExpDisplay() string {
	month := strings.TrimSpace(c.ExpMonth)
	year := strings.TrimSpace(c.ExpYear)

	if month == "" && year == "" {
		return ""
	}

	// Normalize month to MM
	if len(month) == 1 {
		month = "0" + month
	}
	if len(month) > 2 {
		month = month[len(month)-2:]
	}

	// Normalize year to YY
	if len(year) == 4 {
		year = year[2:]
	}
	if len(year) == 1 {
		year = "0" + year
	}

	return month + "/" + year
}

func Load(path string, index int) (Card, error) {
	f, err := os.Open(path)
	if err != nil {
		return Card{}, fmt.Errorf("open cards csv: %w", err)
	}
	defer f.Close()

	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return Card{}, fmt.Errorf("read cards csv: %w", err)
	}
	if len(rows) < 2 {
		return Card{}, fmt.Errorf("no cards found in %s", path)
	}

	header := rows[0]
	dataRows := rows[1:]
	if index >= len(dataRows) {
		return Card{}, fmt.Errorf("card index %d out of range (have %d)", index, len(dataRows))
	}

	row := mapRow(header, dataRows[index])
	expMonth := row["exp_month"]
	expYear := row["exp_year"]

	// Primary format: exp=06/30
	if combined := first(row, "exp", "expiration", "exp_date"); combined != "" {
		if strings.Contains(combined, "/") {
			parts := strings.SplitN(combined, "/", 2)
			expMonth = strings.TrimSpace(parts[0])
			if len(parts) > 1 {
				expYear = strings.TrimSpace(parts[1])
			}
		} else if len(combined) == 4 {
			// MMYY e.g. 0630
			expMonth = combined[:2]
			expYear = combined[2:]
		}
	}

	card := Card{
		Number:   first(row, "card_number", "number"),
		ExpMonth: expMonth,
		ExpYear:  expYear,
		CVV:      first(row, "cvv", "cvc"),
		Name:     first(row, "name", "cardholder"),
		Address:  first(row, "address", "street"),
		City:     row["city"],
		State:    row["state"],
		Zip:      first(row, "zip", "zip_code", "postal"),
		Phone:    row["phone"],
		Email:    first(row, "email", "profile"),
		Account:  row["account"],
	}

	if card.Number == "" {
		return Card{}, fmt.Errorf("card number missing in csv row %d", index)
	}
	return card, nil
}

func LoadAll(path string) ([]Card, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open cards csv: %w", err)
	}
	defer f.Close()

	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read cards csv: %w", err)
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("no cards found in %s", path)
	}

	header := rows[0]
	var out []Card
	for i, values := range rows[1:] {
		row := mapRow(header, values)
		expMonth := row["exp_month"]
		expYear := row["exp_year"]
		if combined := first(row, "exp", "expiration", "exp_date"); combined != "" {
			if strings.Contains(combined, "/") {
				parts := strings.SplitN(combined, "/", 2)
				expMonth = strings.TrimSpace(parts[0])
				if len(parts) > 1 {
					expYear = strings.TrimSpace(parts[1])
				}
			}
		}
		card := Card{
			Number:   first(row, "card_number", "number"),
			ExpMonth: expMonth,
			ExpYear:  expYear,
			CVV:      first(row, "cvv", "cvc"),
			Name:     first(row, "name", "cardholder"),
			Address:  first(row, "address", "street"),
			City:     row["city"],
			State:    row["state"],
			Zip:      first(row, "zip", "zip_code", "postal"),
			Phone:    row["phone"],
			Email:    first(row, "email", "profile"),
			Account:  row["account"],
		}
		if card.Number == "" {
			return nil, fmt.Errorf("card number missing in csv row %d", i)
		}
		out = append(out, card)
	}
	return out, nil
}

func mapRow(header, values []string) map[string]string {
	out := make(map[string]string, len(header))
	for i, key := range header {
		key = strings.TrimSpace(strings.ToLower(key))
		if i < len(values) {
			out[key] = strings.TrimSpace(values[i])
		}
	}
	return out
}

func first(row map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := row[k]; v != "" {
			return v
		}
	}
	return ""
}
