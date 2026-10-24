package config

import "testing"

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoad(t *testing.T) {
	cfg, err := Load(env(map[string]string{"PORT": "9000", "LEDGERLITE_API_KEYS": "k:u"}))
	if err != nil || cfg.Addr != ":9000" || cfg.APIKeys["k"] != "u" {
		t.Fatalf("Load = %+v, %v", cfg, err)
	}
}

func TestLoadRejectsBadInput(t *testing.T) {
	if _, err := Load(env(map[string]string{"PORT": "abc"})); err == nil {
		t.Error("expected invalid PORT to fail")
	}
	if _, err := Load(env(map[string]string{"APP_ENV": "production"})); err == nil {
		t.Error("expected missing keys in production to fail")
	}
}
