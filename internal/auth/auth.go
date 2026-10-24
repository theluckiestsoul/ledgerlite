package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
)

// ParseKeys parses "key1:user1,key2:user2".
func ParseKeys(raw string) (map[string]string, error) {
	keys := make(map[string]string)
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		key, user, ok := strings.Cut(entry, ":")
		if !ok || key == "" || user == "" {
			return nil, errors.New(`api keys must look like "key:userID"`)
		}
		if _, dup := keys[key]; dup {
			return nil, errors.New("duplicate api key")
		}
		keys[key] = user
	}
	return keys, nil
}

type entry struct {
	digest [sha256.Size]byte
	userID string
}

type Authenticator struct {
	entries []entry
}

func New(keys map[string]string) *Authenticator {
	a := &Authenticator{}
	for key, user := range keys {
		a.entries = append(a.entries, entry{digest: sha256.Sum256([]byte(key)), userID: user})
	}
	return a
}

// UserID compares digests in constant time and checks every key, so timing doesn't leak key prefixes.
func (a *Authenticator) UserID(r *http.Request) (string, bool) {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return "", false
	}
	presented := sha256.Sum256([]byte(strings.TrimSpace(token)))
	userID := ""
	for _, e := range a.entries {
		if subtle.ConstantTimeCompare(e.digest[:], presented[:]) == 1 {
			userID = e.userID
		}
	}
	return userID, userID != ""
}
