package api

import (
	"net/http"

	"github.com/theluckiestsoul/ledgerlite/internal/auth"
	"github.com/theluckiestsoul/ledgerlite/internal/ledger"
	"github.com/theluckiestsoul/ledgerlite/internal/webhooks"
)

type Deps struct {
	APIKeys map[string]string
	Store   *ledger.Store
	Now     ledger.Clock
	Webhook webhooks.Options
}

type server struct {
	auth     *auth.Authenticator
	accounts *ledger.Service
	webhooks *webhooks.Registry
	notifier *webhooks.Dispatcher
}

type authedHandler func(w http.ResponseWriter, r *http.Request, userID string) error

func NewServer(d Deps) http.Handler {
	if d.Store == nil {
		d.Store = ledger.NewStore()
	}
	registry := webhooks.NewRegistry(d.Webhook.AllowPrivateNetworks)
	s := &server{
		auth:     auth.New(d.APIKeys),
		accounts: ledger.NewService(d.Store, d.Now),
		webhooks: registry,
		notifier: webhooks.NewDispatcher(registry, d.Webhook),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.Handle("POST /accounts", s.authed(s.openAccount))
	mux.Handle("GET /accounts", s.authed(s.listAccounts))
	mux.Handle("GET /accounts/{id}", s.authed(s.getAccount))
	mux.Handle("POST /accounts/{id}/deposits", s.authed(s.deposit))
	mux.Handle("POST /accounts/{id}/withdrawals", s.authed(s.withdraw))
	mux.Handle("POST /webhooks", s.authed(s.registerWebhook))
	mux.Handle("GET /webhooks", s.authed(s.listWebhooks))
	mux.Handle("DELETE /webhooks/{id}", s.authed(s.deleteWebhook))
	return mux
}

func (s *server) authed(h authedHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := s.auth.UserID(r)
		if !ok {
			writeError(w, &apiError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "missing or invalid API key"})
			return
		}
		if err := h(w, r, userID); err != nil {
			writeError(w, err)
		}
	})
}
