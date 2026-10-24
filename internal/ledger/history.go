package ledger

import (
	"encoding/base64"
	"time"
)

const DefaultPageSize = 50

type HistoryQuery struct {
	Limit  int
	Cursor string
	Type   TxType
	From   time.Time
	To     time.Time
}

type HistoryPage struct {
	Data       []Transaction `json:"data"`
	NextCursor *string       `json:"nextCursor"`
}

type History struct {
	store *Store
}

func NewHistory(store *Store) *History {
	return &History{store: store}
}

// List returns an account's ledger entries, newest first.
func (h *History) List(accountID string, q HistoryQuery) HistoryPage {
	after := decodeCursor(q.Cursor)
	txs := h.store.Transactions(accountID)

	matching := []Transaction{}
	for i := len(txs) - 1; i >= 0; i-- {
		tx := txs[i]
		switch {
		case q.Type != "" && tx.Type != q.Type:
		case !q.From.IsZero() && tx.CreatedAt.Before(q.From):
		case !q.To.IsZero() && tx.CreatedAt.After(q.To):
		case !after.IsZero() && !tx.CreatedAt.Before(after):
		default:
			matching = append(matching, tx)
		}
	}

	page := HistoryPage{Data: matching}
	if len(matching) > q.Limit {
		page.Data = matching[:q.Limit]
		cursor := encodeCursor(page.Data[len(page.Data)-1].CreatedAt)
		page.NextCursor = &cursor
	}
	return page
}

func encodeCursor(t time.Time) string {
	return base64.RawURLEncoding.EncodeToString([]byte(t.Format(time.RFC3339Nano)))
}

func decodeCursor(cursor string) time.Time {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339Nano, string(raw))
	return t
}
