package tm

import (
	"fmt"
	"testing"
)

func TestCheckCheckoutExpired(t *testing.T) {
	cases := []struct {
		name   string
		status int
		html   string
		want   bool
	}{
		{
			name:   "404",
			status: 404,
			html:   "<html></html>",
			want:   true,
		},
		{
			name:   "session expired text",
			status: 200,
			html:   `<html><body>Your session has expired</body></html>`,
			want:   true,
		},
		{
			name:   "reserve expired json",
			status: 200,
			html:   `<script id="__NEXT_DATA__" type="application/json">{"props":{"sessionStatus":"RESERVE_EXPIRED"}}</script>`,
			want:   true,
		},
		{
			name:   "valid checkout",
			status: 200,
			html:   `<script id="__NEXT_DATA__" type="application/json">{"props":{"sessionStatus":"ACTIVE","ticketTypeId":"000000000001"}}</script>`,
			want:   false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckCheckoutExpired(tc.status, tc.html)
			got := IsCheckoutExpired(err)
			if got != tc.want {
				t.Fatalf("CheckCheckoutExpired() expired=%v want=%v err=%v", got, tc.want, err)
			}
		})
	}
}

func TestGraphQLErrorExpired(t *testing.T) {
	err := fmt.Errorf("SDK GraphQL initPayment failed: [{\"code\":\"U002\",\"message\":\"RESERVE_EXPIRED\"}]")
	if !GraphQLErrorExpired(err) {
		t.Fatal("expected GraphQL expired error")
	}
}
