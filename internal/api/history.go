package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/theluckiestsoul/ledgerlite/internal/ledger"
)

func (s *server) listTransactions(w http.ResponseWriter, r *http.Request, _ string) error {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit == 0 {
		limit = ledger.DefaultPageSize
	}
	from, err := parseTime(q.Get("from"), "from")
	if err != nil {
		return err
	}
	to, err := parseTime(q.Get("to"), "to")
	if err != nil {
		return err
	}

	page := s.history.List(r.PathValue("id"), ledger.HistoryQuery{
		Limit:  limit,
		Cursor: q.Get("cursor"),
		Type:   ledger.TxType(q.Get("type")),
		From:   from,
		To:     to,
	})
	writeJSON(w, http.StatusOK, page)
	return nil
}

func parseTime(value, field string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.DateOnly} {
		if t, err := time.Parse(layout, value); err == nil {
			return t, nil
		}
	}
	return time.Time{}, badRequest(field+" must be an RFC 3339 timestamp or YYYY-MM-DD date", field)
}
