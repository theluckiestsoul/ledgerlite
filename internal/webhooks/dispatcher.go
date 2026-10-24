package webhooks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"syscall"
	"time"

	"github.com/theluckiestsoul/ledgerlite/internal/ledger"
)

type Options struct {
	Client      *http.Client
	Sleep       func(time.Duration)
	MaxAttempts int
	BaseDelay   time.Duration
	Workers     int
	QueueSize   int
	// AllowPrivateNetworks permits http and private/loopback targets (local development and tests only).
	AllowPrivateNetworks bool
}

type job struct {
	endpoint Endpoint
	event    Event
}

// Dispatcher delivers events from a background queue so API requests never wait on receivers.
type Dispatcher struct {
	registry    *Registry
	client      *http.Client
	sleep       func(time.Duration)
	maxAttempts int
	baseDelay   time.Duration
	queue       chan job
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
		d.client = safeClient(opts.AllowPrivateNetworks)
	}
	if d.sleep == nil {
		d.sleep = time.Sleep
	}
	if d.maxAttempts <= 0 {
		d.maxAttempts = 3
	}
	if d.baseDelay <= 0 {
		d.baseDelay = 500 * time.Millisecond
	}
	if opts.Workers <= 0 {
		opts.Workers = 4
	}
	if opts.QueueSize <= 0 {
		opts.QueueSize = 1000
	}
	d.queue = make(chan job, opts.QueueSize)
	for i := 0; i < opts.Workers; i++ {
		go d.work()
	}
	return d
}

func (d *Dispatcher) TransactionCreated(ownerID string, tx ledger.Transaction) {
	event := Event{ID: ledger.NewID("evt"), Type: "transaction.created", CreatedAt: time.Now().UTC(), Data: tx}
	for _, ep := range d.registry.endpointsFor(ownerID) {
		select {
		case d.queue <- job{endpoint: ep, event: event}:
		default:
			log.Printf("webhook queue full, dropping event=%s endpoint=%s", event.ID, ep.ID)
		}
	}
}

func (d *Dispatcher) work() {
	for j := range d.queue {
		d.deliver(j.endpoint, j.event)
	}
}

func (d *Dispatcher) deliver(ep Endpoint, event Event) bool {
	payload, err := json.Marshal(event)
	if err != nil {
		log.Printf("webhook marshal failed: event=%s err=%v", event.ID, err)
		return false
	}

	for attempt := 1; attempt <= d.maxAttempts; attempt++ {
		// Queued jobs outlive DELETE /webhooks, so re-check before every attempt.
		if !d.registry.active(ep.ID) {
			log.Printf("webhook endpoint deleted, dropping event=%s endpoint=%s", event.ID, ep.ID)
			return false
		}
		status, err := d.send(ep, payload)
		if err == nil && status >= 200 && status < 300 {
			return true
		}
		log.Printf("webhook delivery failed: endpoint=%s event=%s attempt=%d/%d status=%d err=%v",
			ep.ID, event.ID, attempt, d.maxAttempts, status, err)
		if !retryable(status, err) || attempt == d.maxAttempts {
			return false
		}
		d.sleep(d.baseDelay << (attempt - 1))
	}
	return false
}

func (d *Dispatcher) send(ep Endpoint, payload []byte) (int, error) {
	req, err := http.NewRequest(http.MethodPost, ep.URL, bytes.NewReader(payload))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "ledgerlite-webhooks/1")
	req.Header.Set(SignatureHeader, Sign(ep.Secret, payload, time.Now().Unix()))

	resp, err := d.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	return resp.StatusCode, nil
}

func retryable(status int, err error) bool {
	return err != nil || status >= 500 || status == http.StatusRequestTimeout || status == http.StatusTooManyRequests
}

var errBlockedAddress = errors.New("webhook target resolves to a non-public address")

// safeClient times out and re-checks the resolved IP at dial time, which also covers DNS rebinding.
func safeClient(allowPrivate bool) *http.Client {
	dialer := &net.Dialer{
		Timeout: 3 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			if allowPrivate {
				return nil
			}
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			if ip := net.ParseIP(host); ip == nil || !isPublicIP(ip) {
				return fmt.Errorf("%w: %s", errBlockedAddress, host)
			}
			return nil
		},
	}
	return &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.DialContext(ctx, network, addr)
			},
			TLSHandshakeTimeout: 3 * time.Second,
			MaxIdleConnsPerHost: 4,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}
