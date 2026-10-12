package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/generated/vocabulary/sqlc"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// Searching and paging a learner's own words.
//
// Three hundred words in one upload is a page nobody scrolls through, and a
// learner with a dozen uploads asking "did I add this one already?" should not
// have to open each. One query serves both: every word the learner has pasted,
// optionally narrowed to one upload, to some statuses, or to a substring.

// maxItemQueryLength bounds the search text; a term is at most 200 characters
// and nobody searches with a paragraph.
const maxItemQueryLength = 100

// itemStatuses are the statuses a search may filter on.
var itemStatuses = map[string]bool{
	statusPending: true, statusVerified: true, statusRejected: true,
	statusFailed: true, statusQueued: true,
}

// ItemQuery narrows a search of the learner's words. Zero values mean no filter.
type ItemQuery struct {
	UploadID *uuid.UUID
	Query    string
	Statuses []string
	Limit    int32
	Cursor   string
}

// ItemPage is one page of a learner's words.
type ItemPage struct {
	Items      []UploadItem
	NextCursor string
	HasMore    bool
	Limit      int
}

// SearchItems returns the learner's words, newest upload first and
// alphabetical within one, a page at a time.
func (u *Uploads) SearchItems(ctx context.Context, userID uuid.UUID, query ItemQuery) (ItemPage, error) {
	text := strings.TrimSpace(query.Query)
	if utf8.RuneCountInString(text) > maxItemQueryLength {
		return ItemPage{}, apperr.New(apperr.BadRequest, "INVALID_QUERY",
			"Search text must be at most 100 characters.")
	}
	// Never nil: the query reads an empty array as "no filter", and a nil slice
	// would reach it as NULL and match nothing.
	statuses := make([]string, 0, len(query.Statuses))
	for _, status := range query.Statuses {
		if !itemStatuses[status] {
			return ItemPage{}, apperr.New(apperr.BadRequest, "INVALID_STATUS",
				"Status must be one of pending, verified, rejected, failed or queued.")
		}
		statuses = append(statuses, status)
	}

	params := sqlc.SearchUserUploadItemsParams{
		UserID:   userID,
		UploadID: query.UploadID,
		Statuses: statuses,
		Query:    escapeLike(text),
		RowLimit: query.Limit + 1,
	}
	if query.Cursor != "" {
		position, err := decodeItemCursor(query.Cursor)
		if err != nil {
			return ItemPage{}, err
		}
		params.AfterCreatedAt, params.AfterTerm, params.AfterID = &position.CreatedAt, &position.Term, &position.ID
	}

	rows, err := u.repo.SearchUserUploadItems(ctx, params)
	if err != nil {
		return ItemPage{}, err
	}
	page := ItemPage{Limit: int(query.Limit), Items: make([]UploadItem, 0, len(rows))}
	if len(rows) > int(query.Limit) {
		rows, page.HasMore = rows[:query.Limit], true
	}
	for _, row := range rows {
		page.Items = append(page.Items, toUploadItem(sqlc.ListUploadItemsRow(row)))
	}
	if page.HasMore {
		last := rows[len(rows)-1]
		page.NextCursor = encodeCursor(cursorPosition{CreatedAt: last.CreatedAt, Term: last.Term, ID: last.ID})
	}
	return page, nil
}

// escapeLike makes the learner's text match literally inside ILIKE '%…%'.
func escapeLike(text string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(text)
}

// cursorPosition is where a page ended. Term is set only for word pages.
type cursorPosition struct {
	CreatedAt time.Time `json:"c"`
	Term      string    `json:"t,omitempty"`
	ID        uuid.UUID `json:"i"`
}

func encodeCursor(position cursorPosition) string {
	raw, _ := json.Marshal(position) //nolint:errchkjson // a time, a string and a UUID always marshal
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeCursor(cursor string) (cursorPosition, error) {
	var position cursorPosition
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err == nil {
		err = json.Unmarshal(raw, &position)
	}
	if err != nil || position.ID == uuid.Nil || position.CreatedAt.IsZero() {
		return cursorPosition{}, apperr.New(apperr.BadRequest, "INVALID_CURSOR",
			"Pagination cursor is not decodable.")
	}
	return position, nil
}

func decodeUploadCursor(cursor string) (cursorPosition, error) {
	return decodeCursor(cursor)
}

func decodeItemCursor(cursor string) (cursorPosition, error) {
	position, err := decodeCursor(cursor)
	if err == nil && position.Term == "" {
		return cursorPosition{}, apperr.New(apperr.BadRequest, "INVALID_CURSOR",
			"Pagination cursor is not decodable.")
	}
	return position, err
}
