package api

import "testing"

func TestEmailsFromCookies(t *testing.T) {
	account, profile := EmailsFromCookies([]Cookie{
		{Name: "id_token", Value: "eyJhbGciOiJub25lIn0.eyJlbWFpbCI6ImFjY3RAZXhhbXBsZS5jb20ifQ.sig"},
		{Name: "other", Value: "profile%40example.com"},
	})
	if account != "acct@example.com" {
		t.Fatalf("account=%q", account)
	}
	if profile != "profile@example.com" {
		t.Fatalf("profile=%q", profile)
	}
}
