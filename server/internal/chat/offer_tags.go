package chat

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

const offerTagBatch = 500

// OfferIconResolver maps a parsed item to a local icon URL for the server's
// game-data profile. It returns an empty string when nothing matches.
type OfferIconResolver func(server string, item OfferItem) string

// SetOfferIconResolver installs the optional icon lookup used by Offers.
func (s *Store) SetOfferIconResolver(resolver OfferIconResolver) { s.iconResolver = resolver }

// ensureOfferTags parses Global advertisements that have no tags for the
// current parser version. It runs lazily from Offers, in bounded batches, so
// new messages and parser upgrades need no write-path hook or startup job.
func (s *Store) ensureOfferTags(ctx context.Context) error {
	s.tagMu.Lock()
	defer s.tagMu.Unlock()
	for {
		rows, err := s.pool.Query(ctx, `
SELECT m.message_id::text, m.message FROM chat_messages m
LEFT JOIN global_offer_parse p ON p.message_id=m.message_id
WHERE m.channel='global' AND m.direction='inbound' AND m.event_id IS NOT NULL
	AND (p.message_id IS NULL OR p.parser_version<$1)
ORDER BY m.occurred_at DESC LIMIT $2`, OfferParserVersion, offerTagBatch)
		if err != nil {
			return fmt.Errorf("select unparsed offers: %w", err)
		}
		type pending struct{ id, text string }
		var batch []pending
		for rows.Next() {
			var item pending
			if err := rows.Scan(&item.id, &item.text); err != nil {
				rows.Close()
				return err
			}
			batch = append(batch, item)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if len(batch) == 0 {
			return nil
		}
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return err
		}
		pgBatch := &pgx.Batch{}
		for _, message := range batch {
			pgBatch.Queue(`DELETE FROM global_offer_items WHERE message_id=$1::uuid`, message.id)
			for ordinal, item := range ParseOfferText(message.text) {
				notes := item.Notes
				if notes == nil {
					notes = []string{}
				}
				pgBatch.Queue(`INSERT INTO global_offer_items(message_id,ordinal,item_type,subtype,part,degree,race,seal,plus,price,currency,notes)
VALUES($1::uuid,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, message.id, ordinal, item.Type, item.Subtype, item.Part,
					item.Degree, item.Race, item.Seal, item.Plus, item.Price, item.Currency, notes)
			}
			pgBatch.Queue(`INSERT INTO global_offer_parse(message_id,parser_version) VALUES($1::uuid,$2)
ON CONFLICT(message_id) DO UPDATE SET parser_version=EXCLUDED.parser_version`, message.id, OfferParserVersion)
		}
		results := tx.SendBatch(ctx, pgBatch)
		for i := 0; i < pgBatch.Len(); i++ {
			if _, err := results.Exec(); err != nil {
				results.Close()
				tx.Rollback(ctx)
				return fmt.Errorf("store offer tags: %w", err)
			}
		}
		if err := results.Close(); err != nil {
			tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		if len(batch) < offerTagBatch {
			return nil
		}
	}
}

// offerItems loads the stored tags for a page of offers in one query.
func (s *Store) offerItems(ctx context.Context, tx pgx.Tx, offers []Offer) error {
	if len(offers) == 0 {
		return nil
	}
	ids := make([]string, len(offers))
	index := make(map[string]int, len(offers))
	for i, offer := range offers {
		ids[i] = offer.MessageID
		index[offer.MessageID] = i
	}
	rows, err := tx.Query(ctx, `SELECT message_id::text,item_type,subtype,part,degree,race,seal,plus,price,currency,notes
FROM global_offer_items WHERE message_id=ANY($1::uuid[]) ORDER BY message_id,ordinal`, ids)
	if err != nil {
		return fmt.Errorf("query offer tags: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var item OfferItem
		var degree, plus int16
		if err := rows.Scan(&id, &item.Type, &item.Subtype, &item.Part, &degree, &item.Race, &item.Seal, &plus,
			&item.Price, &item.Currency, &item.Notes); err != nil {
			return err
		}
		item.Degree, item.Plus = int(degree), int(plus)
		if len(item.Notes) == 0 {
			item.Notes = nil
		}
		i := index[id]
		if s.iconResolver != nil {
			item.IconURL = s.iconResolver(offers[i].Server, item)
		}
		offers[i].Items = append(offers[i].Items, item)
	}
	return rows.Err()
}
