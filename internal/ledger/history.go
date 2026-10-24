package ledger

import (
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"
)

const DefaultPageSize = 50

var ErrInvalidCursor = errors.New("cursor is invalid")

type HistoryQuery struct {
	Limit int
	// Cursor is the nextCursor from a previous page.
	Cursor string
	Type   TxType
	From   time.Time // inclusive
	Until  time.Time // exclusive
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

// List returns an account's ledger entries, newest first. Cursors are ledger positions, so equal timestamps are never skipped.
func (h *History) List(accountID string, q HistoryQuery) (HistoryPage, error) {
	before := -1
	if q.Cursor != "" {
		seq, err := decodeCursor(q.Cursor)
		if err != nil {
			return HistoryPage{}, err
		}
		before = seq
	}

	page := HistoryPage{Data: []Transaction{}}
	lastSeq, more := -1, false
	h.store.ScanNewest(accountID, before, func(seq int, tx Transaction) bool {
		if !q.matches(tx) {
			return true
		}
		if len(page.Data) == q.Limit {
			more = true
			return false
		}
		page.Data = append(page.Data, tx)
		lastSeq = seq
		return true
	})

	if more {
		cursor := encodeCursor(lastSeq)
		page.NextCursor = &cursor
	}
	return page, nil
}

func (q HistoryQuery) matches(tx Transaction) bool {
	switch {
	case q.Type != "" && tx.Type != q.Type:
		return false
	case !q.From.IsZero() && tx.CreatedAt.Before(q.From):
		return false
	case !q.Until.IsZero() && !tx.CreatedAt.Before(q.Until):
		return false
	}
	return true
}

func encodeCursor(seq int) string {
	return base64.RawURLEncoding.EncodeToString([]byte("s:" + strconv.Itoa(seq)))
}

func decodeCursor(cursor string) (int, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, ErrInvalidCursor
	}
	n, err := strconv.Atoi(strings.TrimPrefix(string(raw), "s:"))
	if err != nil || n < 0 || !strings.HasPrefix(string(raw), "s:") {
		return 0, ErrInvalidCursor
	}
	return n, nil
}
