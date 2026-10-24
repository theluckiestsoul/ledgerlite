package config

import (
	"errors"
	"fmt"
	"log"
	"strconv"

	"github.com/theluckiestsoul/ledgerlite/internal/auth"
)

const devKeys = "dev_key_alice:user_alice,dev_key_bob:user_bob"

type Config struct {
	Addr    string
	APIKeys map[string]string
}

func Load(getenv func(string) string) (Config, error) {
	port := getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if n, err := strconv.Atoi(port); err != nil || n < 0 || n > 65535 {
		return Config{}, fmt.Errorf("invalid PORT %q", port)
	}

	raw := getenv("LEDGERLITE_API_KEYS")
	if raw == "" {
		if getenv("APP_ENV") == "production" {
			return Config{}, errors.New("LEDGERLITE_API_KEYS must be set in production")
		}
		log.Println("LEDGERLITE_API_KEYS not set, using demo keys (dev_key_alice, dev_key_bob)")
		raw = devKeys
	}
	keys, err := auth.ParseKeys(raw)
	if err != nil {
		return Config{}, err
	}
	return Config{Addr: ":" + port, APIKeys: keys}, nil
}
