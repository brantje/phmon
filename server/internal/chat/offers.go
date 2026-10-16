package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type OfferFilter struct {
	Server string
	Sender string
	Query  string
	Kind   string
	// ItemType, Subtype and Degree match heuristic tags parsed from the text.
	ItemType string
	Subtype  string
	Degree   int
	Cursor   string
	From     *time.Time
	To       *time.Time
	Limit    int
}

type Offer struct {
	MessageID     string      `json:"message_id"`
	SourceEventID string      `json:"source_event_id"`
	Server        string      `json:"server"`
	ObserverID    string      `json:"observer_character_id"`
	Observer      string      `json:"observer"`
	Sender        string      `json:"sender"`
	Channel       string      `json:"channel"`
	RawType       string      `json:"raw_type"`
	Text          string      `json:"message"`
	Kind          string      `json:"offer_type"`
	OccurredAt    time.Time   `json:"occurred_at"`
	Items         []OfferItem `json:"items"`
}

type OfferPage struct {
	Offers []Offer `json:"offers"`
	Total  int64   `json:"total"`
	// Counts holds WTB/WTS/WTT totals under every filter except the offer type.
	Counts     map[string]int64 `json:"counts"`
	NextCursor string           `json:"next_cursor,omitempty"`
}

// GlobalOfferType classifies explicit global advertisements only. A prefix is
// evidence of an advertised intention, not of any completed transaction.
func GlobalOfferType(text string) string {
	trimmed := strings.ToUpper(strings.TrimLeft(text, " \t\r\n"))
	for _, kind := range []string{"WTB", "WTS", "WTT"} {
		if strings.HasPrefix(trimmed, kind) {
			after := strings.TrimPrefix(trimmed, kind)
			if after == "" || strings.ContainsRune(" \t\r\n:,-.", rune(after[0])) {
				return kind
			}
		}
	}
	return "unclassified"
}

func validOfferFilter(filter OfferFilter) bool {
	if filter.Limit < 1 || filter.Limit > MaxPageSize ||
		len(filter.Server) > 100 || len(filter.Sender) > 64 || len(filter.Query) > 128 ||
		len(filter.Cursor) > 256 || len(filter.ItemType) > 32 || len(filter.Subtype) > 32 ||
		filter.Degree < 0 || filter.Degree > 14 {
		return false
	}
	switch filter.Kind {
	case "", "WTB", "WTS", "WTT", "unclassified":
	default:
		return false
	}
	return filter.From == nil || filter.To == nil || filter.To.After(*filter.From)
}

const offerKindSQL = `CASE
	WHEN m.message ~* '^[[:space:]]*WTB([[:space:]:,.-]|$)' THEN 'WTB'
	WHEN m.message ~* '^[[:space:]]*WTS([[:space:]:,.-]|$)' THEN 'WTS'
	WHEN m.message ~* '^[[:space:]]*WTT([[:space:]:,.-]|$)' THEN 'WTT'
	ELSE 'unclassified' END`

// Offers is a read-only query over retained Slice 6 Global chat messages.
// Retained messages have per-observer IDs, not a shared server broadcast ID.
// The same server/sender/raw type/text seen by a different observer within two
// seconds of an earlier copy is treated as that earlier broadcast and omitted;
// repeats from the same observer stay distinct. The earliest copy is kept.
func (s *Store) Offers(ctx context.Context, filter OfferFilter) (OfferPage, error) {
	if s == nil || s.pool == nil || !validOfferFilter(filter) {
		return OfferPage{}, ErrInvalid
	}
	var cursor *Cursor
	if filter.Cursor != "" {
		var err error
		cursor, err = DecodeCursor(filter.Cursor)
		if err != nil {
			return OfferPage{}, err
		}
	}
	var beforeAt, beforeID any
	if cursor != nil {
		beforeAt, beforeID = cursor.At, cursor.ID
	}
	if err := s.ensureOfferTags(ctx); err != nil {
		return OfferPage{}, err
	}
	// Everything except the offer-type clause, shared by the page, its total
	// and the per-type tab counts.
	common := `FROM chat_messages m
	WHERE $4::text IS NOT NULL AND m.channel='global' AND m.direction='inbound'
		AND m.event_id IS NOT NULL AND m.echo_of_command_id IS NULL
		AND m.sender<>'' AND m.raw_type<>''
		AND ($1='' OR lower(m.server_name)=lower($1))
		AND ($2='' OR m.sender ILIKE '%'||$2||'%')
		AND ($3='' OR m.message ILIKE '%'||$3||'%')
		AND ($5::timestamptz IS NULL OR m.occurred_at >= $5)
		AND ($6::timestamptz IS NULL OR m.occurred_at < $6)
		AND ($7='' AND $8='' AND $9=0 OR EXISTS (SELECT 1 FROM global_offer_items t
			WHERE t.message_id=m.message_id AND ($7='' OR t.item_type=$7)
				AND ($8='' OR t.subtype=$8) AND ($9=0 OR t.degree=$9)))
		AND NOT EXISTS (SELECT 1 FROM chat_messages p
			WHERE p.channel='global' AND p.direction='inbound'
				AND p.event_id IS NOT NULL AND p.echo_of_command_id IS NULL
				AND p.character_id<>m.character_id
				AND lower(p.server_name)=lower(m.server_name)
				AND p.raw_type=m.raw_type AND p.sender=m.sender
				AND p.message=m.message
				AND p.occurred_at BETWEEN m.occurred_at-interval '2 seconds' AND m.occurred_at
				AND (p.occurred_at<m.occurred_at OR p.message_id<m.message_id))
`
	base := common + `	AND (($4='' AND (` + offerKindSQL + `)<>'unclassified') OR ($4<>'' AND (` + offerKindSQL + `)=$4))
`
	args := []any{filter.Server, filter.Sender, filter.Query, filter.Kind, filter.From, filter.To,
		filter.ItemType, filter.Subtype, filter.Degree}
	// Count and page the same set of retained observer messages even while
	// agents append new Global chat records concurrently.
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return OfferPage{}, fmt.Errorf("begin global offers: %w", err)
	}
	defer tx.Rollback(ctx)
	var total int64
	if err := tx.QueryRow(ctx, `SELECT count(*) `+base, args...).Scan(&total); err != nil {
		return OfferPage{}, fmt.Errorf("count global offers: %w", err)
	}
	counts := map[string]int64{"WTB": 0, "WTS": 0, "WTT": 0}
	countRows, err := tx.Query(ctx, `SELECT kind, count(*) FROM (SELECT (`+offerKindSQL+`) AS kind `+common+`) k
WHERE kind<>'unclassified' GROUP BY kind`, args...)
	if err != nil {
		return OfferPage{}, fmt.Errorf("count global offer types: %w", err)
	}
	for countRows.Next() {
		var kind string
		var n int64
		if err := countRows.Scan(&kind, &n); err != nil {
			countRows.Close()
			return OfferPage{}, err
		}
		counts[kind] = n
	}
	countRows.Close()
	if err := countRows.Err(); err != nil {
		return OfferPage{}, err
	}
	rows, err := tx.Query(ctx, `SELECT m.message_id::text,m.event_id::text,m.server_name,
		m.character_id::text,m.character_name,m.sender,m.channel,m.raw_type,
		m.message,`+offerKindSQL+`,m.occurred_at `+base+`
		AND ($10::timestamptz IS NULL OR
			(m.occurred_at,m.message_id)<($10::timestamptz,$11::uuid))
		ORDER BY m.occurred_at DESC,m.message_id DESC LIMIT $12`,
		append(args, beforeAt, beforeID, filter.Limit+1)...)
	if err != nil {
		return OfferPage{}, fmt.Errorf("query global offers: %w", err)
	}
	defer rows.Close()
	page := OfferPage{Offers: make([]Offer, 0, filter.Limit), Total: total, Counts: counts}
	for rows.Next() {
		var offer Offer
		if err := rows.Scan(&offer.MessageID, &offer.SourceEventID, &offer.Server,
			&offer.ObserverID, &offer.Observer, &offer.Sender, &offer.Channel,
			&offer.RawType, &offer.Text, &offer.Kind, &offer.OccurredAt); err != nil {
			return OfferPage{}, err
		}
		page.Offers = append(page.Offers, offer)
	}
	if err := rows.Err(); err != nil {
		return OfferPage{}, err
	}
	if len(page.Offers) > filter.Limit {
		page.Offers = page.Offers[:filter.Limit]
		last := page.Offers[len(page.Offers)-1]
		page.NextCursor = EncodeCursor(last.OccurredAt, last.MessageID)
	}
	rows.Close()
	if err := s.offerItems(ctx, tx, page.Offers); err != nil {
		return OfferPage{}, err
	}
	for i := range page.Offers {
		if page.Offers[i].Items == nil {
			page.Offers[i].Items = []OfferItem{}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return OfferPage{}, fmt.Errorf("commit global offers snapshot: %w", err)
	}
	return page, nil
}
