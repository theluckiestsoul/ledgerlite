package api

import (
	"net/http"

	"github.com/theluckiestsoul/ledgerlite/internal/auth"
	"github.com/theluckiestsoul/ledgerlite/internal/ledger"
)

type Deps struct {
	APIKeys map[string]string
	Store   *ledger.Store
	Now     ledger.Clock
}

type server struct {
	auth        *auth.Authenticator
	accounts    *ledger.Service
	idempotency map[string]cachedResponse
}

type authedHandler func(w http.ResponseWriter, r *http.Request, userID string) error

func NewServer(d Deps) http.Handler {
	if d.Store == nil {
		d.Store = ledger.NewStore()
	}
	s := &server{
		auth:        auth.New(d.APIKeys),
		accounts:    ledger.NewService(d.Store, d.Now),
		idempotency: make(map[string]cachedResponse),
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
	mux.Handle("POST /transfers", s.authed(s.createTransfer))
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
