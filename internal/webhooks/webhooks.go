package webhooks

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"sync"
	"time"

	"github.com/theluckiestsoul/ledgerlite/internal/ledger"
)

var ErrInvalidURL = errors.New("url must be an absolute http or https URL")

type Endpoint struct {
	ID        string    `json:"id"`
	OwnerID   string    `json:"ownerId"`
	URL       string    `json:"url"`
	Secret    string    `json:"secret"`
	CreatedAt time.Time `json:"createdAt"`
}

type Event struct {
	ID        string             `json:"id"`
	Type      string             `json:"type"`
	CreatedAt time.Time          `json:"createdAt"`
	Data      ledger.Transaction `json:"data"`
}

type Registry struct {
	mu        sync.Mutex
	endpoints []Endpoint
}

func NewRegistry() *Registry {
	return &Registry{}
}

func (r *Registry) Register(ownerID, rawURL string) (Endpoint, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Endpoint{}, ErrInvalidURL
	}
	secret := make([]byte, 24)
	if _, err := rand.Read(secret); err != nil {
		return Endpoint{}, err
	}
	ep := Endpoint{
		ID:        ledger.NewID("whk"),
		OwnerID:   ownerID,
		URL:       u.String(),
		Secret:    "whsec_" + hex.EncodeToString(secret),
		CreatedAt: time.Now().UTC(),
	}
	r.mu.Lock()
	r.endpoints = append(r.endpoints, ep)
	r.mu.Unlock()
	return ep, nil
}

func (r *Registry) List(ownerID string) []Endpoint {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []Endpoint{}
	for _, ep := range r.endpoints {
		if ep.OwnerID == ownerID {
			out = append(out, ep)
		}
	}
	return out
}
