package api

import (
	"errors"
	"net/http"

	"github.com/theluckiestsoul/ledgerlite/internal/webhooks"
)

type registerWebhookRequest struct {
	URL string `json:"url"`
}

func (s *server) registerWebhook(w http.ResponseWriter, r *http.Request, userID string) error {
	var req registerWebhookRequest
	if err := decodeJSON(w, r, &req); err != nil {
		return err
	}
	ep, err := s.webhooks.Register(userID, req.URL)
	switch {
	case errors.Is(err, webhooks.ErrInvalidURL):
		return badRequest(err.Error(), "url")
	case errors.Is(err, webhooks.ErrTooManyEndpoints):
		return &apiError{Status: http.StatusUnprocessableEntity, Code: "webhook_limit_reached", Message: err.Error()}
	case err != nil:
		return err
	}
	writeJSON(w, http.StatusCreated, ep)
	return nil
}

func (s *server) listWebhooks(w http.ResponseWriter, _ *http.Request, userID string) error {
	writeJSON(w, http.StatusOK, map[string]any{"data": s.webhooks.List(userID)})
	return nil
}

func (s *server) deleteWebhook(w http.ResponseWriter, r *http.Request, userID string) error {
	if err := s.webhooks.Delete(userID, r.PathValue("id")); err != nil {
		return &apiError{Status: http.StatusNotFound, Code: "not_found", Message: err.Error()}
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
