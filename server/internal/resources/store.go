package resources

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool     *pgxpool.Pool
	metadata *ItemMetadata
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) Apply(ctx context.Context, agentID, characterID string, generation, revision uint64, sessionID string, snapshot Snapshot) error {
	if s == nil || s.pool == nil || generation == 0 || revision == 0 || snapshot.Revision != revision {
		return ErrInvalid
	}
	observations, hashes, err := Validate(snapshot)
	if err != nil {
		return err
	}
	revisionValue := int64(revision)
	generationValue := int64(generation)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, characterID); err != nil {
		return err
	}
	var currentSession string
	err = tx.QueryRow(ctx, `SELECT session_id::text FROM character_sessions WHERE character_id=$1 AND agent_id=$2 AND connection_generation=$3 AND session_id=$4::uuid AND ended_at IS NULL FOR UPDATE`, characterID, agentID, generationValue, sessionID).Scan(&currentSession)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrStale
	}
	if err != nil {
		return err
	}
	var previousSession string
	var previousRevision int64
	var previousAgent string
	var previousGeneration int64
	err = tx.QueryRow(ctx, `SELECT session_id::text,revision,agent_id::text,connection_generation FROM character_resource_state WHERE character_id=$1 FOR UPDATE`, characterID).Scan(&previousSession, &previousRevision, &previousAgent, &previousGeneration)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		if !snapshot.Full {
			return ErrSequence
		}
	} else if previousSession != sessionID || previousAgent != agentID || previousGeneration != generationValue {
		if !snapshot.Full {
			return ErrSequence
		}
	} else if snapshot.Full {
		if revisionValue <= previousRevision {
			return ErrSequence
		}
	} else if int64(snapshot.BaseRevision) != previousRevision || revisionValue != previousRevision+1 {
		return ErrSequence
	}

	var observerServer, observerGuild string
	if err = tx.QueryRow(ctx, `SELECT server_key,coalesce(guild_name,'') FROM characters WHERE character_id=$1`, characterID).Scan(&observerServer, &observerGuild); err != nil {
		return err
	}
	now := time.Now().UTC()
	for key, observation := range observations {
		var serverKey, guildKey string
		if key == "guild_storage" {
			serverKey, guildKey = observerServer, normalizeGuild(observerGuild)
		} else {
			serverKey = observerServer
		}
		hash := hashes[key]
		var previousHash []byte
		previousErr := tx.QueryRow(ctx, `SELECT content_hash FROM character_resource_observations WHERE observer_character_id=$1 AND resource_key=$2 FOR UPDATE`, characterID, key).Scan(&previousHash)
		if previousErr != nil && !errors.Is(previousErr, pgx.ErrNoRows) {
			return previousErr
		}
		contentChanged := observation.Availability == "observed" && (errors.Is(previousErr, pgx.ErrNoRows) || !bytes.Equal(previousHash, hash[:]))
		_, err = tx.Exec(ctx, `INSERT INTO character_resource_observations (character_id,observer_character_id,server_key,guild_key,resource_key,session_id,revision,availability,payload,content_hash,observed_at,updated_at)
VALUES ($1,$1,$2,$3,$4,$5::uuid,$6,$7,$8::jsonb,$9,CASE WHEN $7='observed' THEN $10::timestamptz ELSE NULL END,now())
ON CONFLICT (observer_character_id,resource_key) DO UPDATE SET
	character_id=EXCLUDED.character_id,server_key=EXCLUDED.server_key,guild_key=EXCLUDED.guild_key,session_id=EXCLUDED.session_id,revision=EXCLUDED.revision,availability=EXCLUDED.availability,
 payload=CASE WHEN EXCLUDED.availability='observed' OR EXCLUDED.resource_key='item_enrichment' THEN EXCLUDED.payload ELSE character_resource_observations.payload END,
 content_hash=CASE WHEN EXCLUDED.availability='observed' OR EXCLUDED.resource_key='item_enrichment' THEN EXCLUDED.content_hash ELSE character_resource_observations.content_hash END,
 observed_at=CASE WHEN EXCLUDED.availability='observed' AND character_resource_observations.content_hash<>EXCLUDED.content_hash THEN EXCLUDED.observed_at ELSE character_resource_observations.observed_at END,
 updated_at=now()`, characterID, serverKey, guildKey, key, sessionID, revisionValue, observation.Availability, observation.Payload, hash[:], now)
		if err != nil {
			return fmt.Errorf("save %s observation: %w", key, err)
		}
		if observation.Availability != "observed" {
			continue
		}
		if contentChanged {
			if err := replaceItemRows(ctx, tx, characterID, serverKey, guildKey, sessionID, revision, key, observation.Payload, now); err != nil {
				return fmt.Errorf("save %s item slots: %w", key, err)
			}
		} else if isItemResource(key) {
			if _, err := tx.Exec(ctx, `UPDATE character_resource_items SET server_key=$3,guild_key=$4,session_id=$5::uuid,revision=$6 WHERE observer_character_id=$1 AND container_key=$2`, characterID, key, serverKey, guildKey, sessionID, revisionValue); err != nil {
				return err
			}
		}
	}
	if snapshot.Full {
		if _, err = tx.Exec(ctx, `DELETE FROM character_resource_items WHERE observer_character_id=$1 AND NOT (container_key = ANY($2::text[]))`, characterID, mapKeys(observations)); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM character_resource_observations WHERE observer_character_id=$1 AND NOT (resource_key = ANY($2::text[]))`, characterID, mapKeys(observations)); err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO character_resource_state (character_id,agent_id,connection_generation,session_id,revision,updated_at) VALUES ($1,$2,$3,$4::uuid,$5,now())
ON CONFLICT (character_id) DO UPDATE SET agent_id=EXCLUDED.agent_id,connection_generation=EXCLUDED.connection_generation,session_id=EXCLUDED.session_id,revision=EXCLUDED.revision,updated_at=now()`, characterID, agentID, generationValue, sessionID, revisionValue)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) Character(ctx context.Context, characterID string, resourceKeys ...string) (View, error) {
	var rows pgx.Rows
	var err error
	if len(resourceKeys) == 0 {
		rows, err = s.pool.Query(ctx, `SELECT resource_key,availability,payload,observed_at,updated_at,revision,server_key FROM character_resource_observations WHERE observer_character_id=$1 ORDER BY resource_key`, characterID)
	} else {
		rows, err = s.pool.Query(ctx, `SELECT resource_key,availability,payload,observed_at,updated_at,revision,server_key FROM character_resource_observations WHERE observer_character_id=$1 AND resource_key=ANY($2::text[]) ORDER BY resource_key`, characterID, resourceKeys)
	}
	if err != nil {
		return View{}, err
	}
	defer rows.Close()
	view := View{CharacterID: characterID, Resources: make(map[string]Observation)}
	for rows.Next() {
		var item Observation
		var observedAt, checkedAt pgtype.Timestamptz
		var revision uint64
		var server string
		if err := rows.Scan(&item.ResourceKey, &item.Availability, &item.Payload, &observedAt, &checkedAt, &revision, &server); err != nil {
			return View{}, err
		}
		item.Revision = revision
		item.Payload = s.metadata.enrich(server, item.Payload)
		if observedAt.Valid {
			item.ObservedAt = observedAt.Time.UTC().Format(time.RFC3339Nano)
		}
		if checkedAt.Valid {
			item.CheckedAt = checkedAt.Time.UTC().Format(time.RFC3339Nano)
			if item.CheckedAt > view.UpdatedAt {
				view.UpdatedAt = item.CheckedAt
			}
		}
		view.Resources[item.ResourceKey] = item
		if revision > view.Revision {
			view.Revision = revision
		}
	}
	if err := rows.Err(); err != nil {
		return View{}, err
	}
	return view, nil
}

func (s *Store) GuildStorage(ctx context.Context, server, guild string) ([]Observation, error) {
	rows, err := s.pool.Query(ctx, `SELECT DISTINCT ON (o.resource_key) o.resource_key,o.availability,o.payload,o.observed_at,o.updated_at,o.revision,o.observer_character_id::text,c.character_name
FROM character_resource_observations o JOIN characters c ON c.character_id=o.observer_character_id
WHERE o.resource_key='guild_storage' AND o.server_key=$1 AND o.guild_key=$2
ORDER BY o.resource_key,o.observed_at DESC NULLS LAST,o.updated_at DESC,o.observer_character_id`, strings.ToLower(strings.TrimSpace(server)), normalizeGuild(guild))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Observation
	for rows.Next() {
		var item Observation
		var observedAt, checkedAt pgtype.Timestamptz
		if err := rows.Scan(&item.ResourceKey, &item.Availability, &item.Payload, &observedAt, &checkedAt, &item.Revision, &item.ObserverCharacterID, &item.ObserverName); err != nil {
			return nil, err
		}
		if observedAt.Valid {
			item.ObservedAt = observedAt.Time.UTC().Format(time.RFC3339Nano)
		}
		if checkedAt.Valid {
			item.CheckedAt = checkedAt.Time.UTC().Format(time.RFC3339Nano)
		}
		item.Payload = s.metadata.enrich(server, item.Payload)
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func normalizeGuild(value string) string { return strings.ToLower(strings.TrimSpace(value)) }

type slotRecord struct {
	SourceSlot    int             `json:"source_slot"`
	DisplayedSlot *int            `json:"displayed_slot,omitempty"`
	Item          json.RawMessage `json:"item"`
}

type petRecord struct {
	PetID string       `json:"pet_id"`
	Slots []slotRecord `json:"slots"`
}

func replaceItemRows(ctx context.Context, tx pgx.Tx, characterID, serverKey, guildKey, sessionID string, revision uint64, resourceKey string, payload json.RawMessage, observedAt time.Time) error {
	if !isItemResource(resourceKey) {
		return nil
	}
	if _, err := tx.Exec(ctx, `DELETE FROM character_resource_items WHERE observer_character_id=$1 AND container_key=$2`, characterID, resourceKey); err != nil {
		return err
	}
	var root struct {
		Slots []slotRecord `json:"slots"`
		Pets  []petRecord  `json:"pets"`
	}
	if err := json.Unmarshal(payload, &root); err != nil {
		return err
	}
	if resourceKey == "pets" {
		for _, pet := range root.Pets {
			if len(pet.PetID) > 64 {
				continue
			}
			for _, slot := range pet.Slots {
				if err := insertItemRow(ctx, tx, characterID, serverKey, guildKey, resourceKey, pet.PetID, slot, sessionID, int64(revision), observedAt); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, slot := range root.Slots {
		if err := insertItemRow(ctx, tx, characterID, serverKey, guildKey, resourceKey, "", slot, sessionID, int64(revision), observedAt); err != nil {
			return err
		}
	}
	return nil
}

func isItemResource(key string) bool {
	switch key {
	case "inventory", "equipment", "storage", "guild_storage", "job_pouch", "pets":
		return true
	default:
		return false
	}
}

func insertItemRow(ctx context.Context, tx pgx.Tx, characterID, serverKey, guildKey, containerKey, petID string, slot slotRecord, sessionID string, revision int64, observedAt time.Time) error {
	if slot.SourceSlot < 0 || len(slot.Item) == 0 || bytes.Equal(bytes.TrimSpace(slot.Item), []byte("null")) {
		return nil
	}
	var item map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(slot.Item))
	decoder.UseNumber()
	if err := decoder.Decode(&item); err != nil || item == nil {
		return nil
	}
	var model, name, serverName, quantity, plus, durability any
	model = exactInteger(item["model"])
	name = boundedString(item["name"], 256)
	serverName = boundedString(item["servername"], 256)
	quantity = exactNumeric(item["quantity"])
	plus = exactNumeric(item["plus"])
	durability = exactNumeric(item["durability"])
	_, err := tx.Exec(ctx, `INSERT INTO character_resource_items
(observer_character_id,owner_character_id,server_key,guild_key,container_key,pet_id,source_slot,displayed_slot,session_id,revision,observed_at,model,name,server_name,quantity,plus,durability,raw_item)
VALUES ($1,$1,$2,$3,$4,$5,$6,$7,$8::uuid,$9,$10,$11,$12,$13,$14,$15,$16,$17::jsonb)`,
		characterID, serverKey, guildKey, containerKey, petID, slot.SourceSlot, slot.DisplayedSlot, sessionID, revision, observedAt, model, name, serverName, quantity, plus, durability, slot.Item)
	return err
}

func exactInteger(raw json.RawMessage) any {
	n := exactNumeric(raw)
	if n == nil {
		return nil
	}
	parsed, err := strconv.ParseInt(n.(string), 10, 64)
	if err != nil {
		return nil
	}
	return parsed
}

func exactNumeric(raw json.RawMessage) any {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err != nil {
		return nil
	}
	value := number.String()
	if len(value) > 80 || strings.ContainsAny(value, "eE") {
		return nil
	}
	return value
}

func boundedString(raw json.RawMessage, max int) any {
	var value string
	if len(raw) == 0 || json.Unmarshal(raw, &value) != nil || len(value) > max {
		return nil
	}
	return value
}

func mapKeys(values map[string]Observation) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

func DecodeObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
	var value map[string]json.RawMessage
	if err := json.Unmarshal(raw, &value); err != nil || value == nil {
		return nil, ErrInvalid
	}
	return value, nil
}
