package chat

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type OfferFilter struct {
	Server string
	Sender string
	Query string
	Kind string
	Cursor string
	From *time.Time
	To *time.Time
	Limit int
}

type Offer struct {
	MessageID string `json:"message_id"`
	SourceEventID string `json:"source_event_id"`
	Server string `json:"server"`
	ObserverID string `json:"observer_character_id"`
	Observer string `json:"observer"`
	Sender string `json:"sender"`
	Channel string `json:"channel"`
	RawType string `json:"raw_type"`
	Text string `json:"message"`
	Kind string `json:"offer_type"`
	OccurredAt time.Time `json:"occurred_at"`
}

type OfferPage struct {
	Offers []Offer `json:"offers"`
	Total int64 `json:"total"`
	NextCursor string `json:"next_cursor,omitempty"`
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
		len(filter.Cursor) > 256 {
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
// Multiple observers are collapsed only for identical source content/raw type
// within 500ms, and only if neither observer reported a competing repeated
// advertisement within two seconds. Ambiguous repeats remain separate.
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
	base := `FROM chat_messages m
	WHERE m.channel='global' AND m.direction='inbound'
		AND m.event_id IS NOT NULL AND m.echo_of_command_id IS NULL
		AND m.sender<>'' AND m.raw_type<>''
		AND ($1='' OR lower(m.server_name)=lower($1))
		AND ($2='' OR m.sender ILIKE '%'||$2||'%')
		AND ($3='' OR m.message ILIKE '%'||$3||'%')
		AND (($4='' AND (` + offerKindSQL + `)<>'unclassified') OR ($4<>'' AND (` + offerKindSQL + `)=$4))
		AND ($5::timestamptz IS NULL OR m.occurred_at >= $5)
		AND ($6::timestamptz IS NULL OR m.occurred_at < $6)
		AND NOT EXISTS (
			SELECT 1 FROM chat_messages other
			WHERE other.channel='global' AND other.direction='inbound'
				AND other.event_id IS NOT NULL AND other.echo_of_command_id IS NULL
				AND other.character_id<>m.character_id
				AND lower(other.server_name)=lower(m.server_name)
				AND other.sender=m.sender AND other.raw_type=m.raw_type
				AND other.message=m.message
				AND other.occurred_at BETWEEN m.occurred_at-interval '500 milliseconds' AND m.occurred_at
				AND (other.occurred_at,other.message_id)<(m.occurred_at,m.message_id)
				AND (SELECT count(*) FROM chat_messages self
					WHERE self.character_id=m.character_id
					AND lower(self.server_name)=lower(m.server_name)
					AND self.channel='global' AND self.direction='inbound'
					AND self.raw_type=m.raw_type AND self.sender=m.sender
					AND self.message=m.message
					AND self.occurred_at BETWEEN m.occurred_at-interval '2 seconds'
						AND m.occurred_at+interval '2 seconds')=1
				AND (SELECT count(*) FROM chat_messages repeated
					WHERE repeated.character_id=other.character_id
					AND lower(repeated.server_name)=lower(m.server_name)
					AND repeated.channel='global' AND repeated.direction='inbound'
					AND repeated.raw_type=m.raw_type AND repeated.sender=m.sender
					AND repeated.message=m.message
					AND repeated.occurred_at BETWEEN other.occurred_at-interval '2 seconds'
						AND other.occurred_at+interval '2 seconds')=1
		)`
	args := []any{filter.Server, filter.Sender, filter.Query, filter.Kind, filter.From, filter.To}
	var total int64
	if err := s.pool.QueryRow(ctx, `SELECT count(*) `+base, args...).Scan(&total); err != nil {
		return OfferPage{}, fmt.Errorf("count global offers: %w", err)
	}
	rows, err := s.pool.Query(ctx, `SELECT m.message_id::text,m.event_id::text,m.server_name,
		m.character_id::text,m.character_name,m.sender,m.channel,m.raw_type,
		m.message,`+offerKindSQL+`,m.occurred_at `+base+`
		AND ($7::timestamptz IS NULL OR
			(m.occurred_at,m.message_id)<($7::timestamptz,$8::uuid))
		ORDER BY m.occurred_at DESC,m.message_id DESC LIMIT $9`,
		append(args,beforeAt,beforeID,filter.Limit+1)...)
	if err != nil {
		return OfferPage{}, fmt.Errorf("query global offers: %w", err)
	}
	defer rows.Close()
	page := OfferPage{Offers: make([]Offer,0,filter.Limit),Total:total}
	for rows.Next() {
		var offer Offer
		if err := rows.Scan(&offer.MessageID,&offer.SourceEventID,&offer.Server,
			&offer.ObserverID,&offer.Observer,&offer.Sender,&offer.Channel,
			&offer.RawType,&offer.Text,&offer.Kind,&offer.OccurredAt); err != nil {
			return OfferPage{}, err
		}
		page.Offers=append(page.Offers,offer)
	}
	if err := rows.Err(); err != nil {
		return OfferPage{}, err
	}
	if len(page.Offers)>filter.Limit {
		page.Offers=page.Offers[:filter.Limit]
		last:=page.Offers[len(page.Offers)-1]
		page.NextCursor=EncodeCursor(last.OccurredAt,last.MessageID)
	}
	return page,nil
}
