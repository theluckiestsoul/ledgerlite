package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/theluckiestsoul/ledgerlite/internal/ledger"
)

const maxPageSize = 200

func (s *server) listTransactions(w http.ResponseWriter, r *http.Request, userID string) error {
	accountID := r.PathValue("id")
	if _, err := s.accounts.Get(userID, accountID); err != nil {
		return err
	}

	q := r.URL.Query()
	limit, err := parseLimit(q.Get("limit"))
	if err != nil {
		return err
	}
	txType, err := parseTxType(q.Get("type"))
	if err != nil {
		return err
	}
	from, err := parseBound(q.Get("from"), "from", false)
	if err != nil {
		return err
	}
	until, err := parseBound(q.Get("to"), "to", true)
	if err != nil {
		return err
	}

	page, err := s.history.List(accountID, ledger.HistoryQuery{
		Limit:  limit,
		Cursor: q.Get("cursor"),
		Type:   txType,
		From:   from,
		Until:  until,
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, page)
	return nil
}

func parseLimit(value string) (int, error) {
	if value == "" {
		return ledger.DefaultPageSize, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 || n > maxPageSize {
		return 0, badRequest("limit must be an integer between 1 and 200", "limit")
	}
	return n, nil
}

func parseTxType(value string) (ledger.TxType, error) {
	switch t := ledger.TxType(value); t {
	case "", ledger.Deposit, ledger.Withdrawal:
		return t, nil
	default:
		return "", badRequest("type must be deposit or withdrawal", "type")
	}
}

// parseBound accepts RFC 3339 or YYYY-MM-DD; as an upper bound, a date covers the whole day.
func parseBound(value, field string, upper bool) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		if upper {
			return t.Add(time.Nanosecond), nil
		}
		return t, nil
	}
	if d, err := time.Parse(time.DateOnly, value); err == nil {
		if upper {
			return d.AddDate(0, 0, 1), nil
		}
		return d, nil
	}
	return time.Time{}, badRequest(field+" must be an RFC 3339 timestamp or YYYY-MM-DD date", field)
}
