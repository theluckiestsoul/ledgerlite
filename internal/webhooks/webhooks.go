package webhooks

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/theluckiestsoul/ledgerlite/internal/ledger"
)

var ErrInvalidURL = errors.New("url must be a public https URL")

type Endpoint struct {
	ID      string `json:"id"`
	OwnerID string `json:"ownerId"`
	URL     string `json:"url"`
	// Secret is only returned once, when the endpoint is registered.
	Secret    string    `json:"secret,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type Event struct {
	ID        string             `json:"id"`
	Type      string             `json:"type"`
	CreatedAt time.Time          `json:"createdAt"`
	Data      ledger.Transaction `json:"data"`
}

type Registry struct {
	mu           sync.Mutex
	endpoints    []Endpoint
	allowPrivate bool
}

// NewRegistry creates a registry; allowPrivate permits http and private addresses (development and tests only).
func NewRegistry(allowPrivate bool) *Registry {
	return &Registry{allowPrivate: allowPrivate}
}

func (r *Registry) Register(ownerID, rawURL string) (Endpoint, error) {
	u, err := validateURL(rawURL, r.allowPrivate)
	if err != nil {
		return Endpoint{}, err
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

// List returns the owner's endpoints without their secrets.
func (r *Registry) List(ownerID string) []Endpoint {
	out := r.endpointsFor(ownerID)
	for i := range out {
		out[i].Secret = ""
	}
	return out
}

func (r *Registry) endpointsFor(ownerID string) []Endpoint {
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

func validateURL(rawURL string, allowPrivate bool) (*url.URL, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Hostname() == "" || u.User != nil {
		return nil, ErrInvalidURL
	}
	switch {
	case u.Scheme == "https":
	case u.Scheme == "http" && allowPrivate:
	default:
		return nil, ErrInvalidURL
	}
	if allowPrivate {
		return u, nil
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".internal") || strings.HasSuffix(host, ".local") {
		return nil, ErrInvalidURL
	}
	if ip := net.ParseIP(host); ip != nil && !isPublicIP(ip) {
		return nil, ErrInvalidURL
	}
	return u, nil
}

var sharedAddressSpace = netip.MustParsePrefix("100.64.0.0/10")

func isPublicIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	if addr, ok := netip.AddrFromSlice(ip); ok && sharedAddressSpace.Contains(addr.Unmap()) {
		return false
	}
	return true
}
