package webhooks

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/theluckiestsoul/ledgerlite/internal/ledger"
)

type Options struct {
	Client      *http.Client
	Sleep       func(time.Duration)
	MaxAttempts int
	BaseDelay   time.Duration
}

type Dispatcher struct {
	registry    *Registry
	client      *http.Client
	sleep       func(time.Duration)
	maxAttempts int
	baseDelay   time.Duration
}

func NewDispatcher(registry *Registry, opts Options) *Dispatcher {
	d := &Dispatcher{
		registry:    registry,
		client:      opts.Client,
		sleep:       opts.Sleep,
		maxAttempts: opts.MaxAttempts,
		baseDelay:   opts.BaseDelay,
	}
	if d.client == nil {
		d.client = http.DefaultClient
	}
	if d.sleep == nil {
		d.sleep = time.Sleep
	}
	if d.maxAttempts == 0 {
		d.maxAttempts = 3
	}
	if d.baseDelay == 0 {
		d.baseDelay = 500 * time.Millisecond
	}
	return d
}

func (d *Dispatcher) TransactionCreated(ownerID string, tx ledger.Transaction) {
	event := Event{ID: ledger.NewID("evt"), Type: "transaction.created", CreatedAt: time.Now().UTC(), Data: tx}
	for _, ep := range d.registry.List(ownerID) {
		d.deliver(ep, event)
	}
}

// deliver retries failed attempts with exponential backoff.
func (d *Dispatcher) deliver(ep Endpoint, event Event) bool {
	payload, err := json.Marshal(event)
	if err != nil {
		log.Printf("marshal webhook event: %v", err)
		return false
	}

	for attempt := 0; attempt <= d.maxAttempts; attempt++ {
		req, err := http.NewRequest(http.MethodPost, ep.URL, bytes.NewReader(payload))
		if err != nil {
			return false
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "ledgerlite-webhooks/1")
		req.Header.Set(SignatureHeader, Sign(ep.Secret, payload, time.Now().Unix()))

		resp, err := d.client.Do(req)
		if err != nil {
			log.Printf("webhook delivery failed: endpoint=%+v event=%s attempt=%d err=%v", ep, event.ID, attempt, err)
		} else if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return true
		}
		d.sleep(d.baseDelay << attempt)
	}
	return false
}
