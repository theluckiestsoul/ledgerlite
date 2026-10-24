package auth

import (
	"net/http/httptest"
	"testing"
)

func TestParseKeys(t *testing.T) {
	keys, err := ParseKeys(" k1:u1, k2:u2 ,")
	if err != nil || len(keys) != 2 || keys["k1"] != "u1" || keys["k2"] != "u2" {
		t.Fatalf("ParseKeys = %v, %v", keys, err)
	}
	for _, bad := range []string{"no-separator", ":user", "key:", "k:u1,k:u2"} {
		if _, err := ParseKeys(bad); err == nil {
			t.Errorf("ParseKeys(%q) should fail", bad)
		}
	}
}

func TestUserID(t *testing.T) {
	a := New(map[string]string{"secret": "user_1"})
	cases := map[string]string{
		"Bearer secret": "user_1",
		"bearer secret": "user_1",
		"Bearer wrong":  "",
		"Basic secret":  "",
		"":              "",
	}
	for header, want := range cases {
		r := httptest.NewRequest("GET", "/", nil)
		if header != "" {
			r.Header.Set("Authorization", header)
		}
		if got, _ := a.UserID(r); got != want {
			t.Errorf("UserID(%q) = %q, want %q", header, got, want)
		}
	}
}
