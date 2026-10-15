package resources

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type ItemSearchFilter struct {
	Server           string
	Query            string
	Container        string
	Owner            string
	Type             string
	Subcategory      string
	Degree           string
	IncludeEquipment bool
	Cursor           string
	Limit            int
}

type ItemSearchEntry struct {
	Server              string         `json:"server"`
	serverKey           string
	Container           string         `json:"container"`
	OwnerKind           string         `json:"owner_kind"`
	OwnerID             string         `json:"owner_id"`
	OwnerName           string         `json:"owner_name,omitempty"`
	Guild               string         `json:"guild,omitempty"`
	ObserverCharacterID string         `json:"observer_character_id"`
	ObserverName        string         `json:"observer_name"`
	PetID               string         `json:"pet_id,omitempty"`
	Slot                int            `json:"slot"`
	Revision            int64          `json:"revision"`
	ObservedAt          time.Time      `json:"observed_at"`
	CheckedAt           time.Time      `json:"checked_at"`
	QuantityExact       string         `json:"quantity_exact,omitempty"`
	Item                map[string]any `json:"item"`
	Metadata            map[string]any `json:"metadata,omitempty"`
}

type ItemSearchPage struct {
	Items      []ItemSearchEntry `json:"items"`
	Total      int64             `json:"total"`
	NextCursor string            `json:"next_cursor,omitempty"`
}

type itemSearchCursor struct {
	Server    string `json:"s"`
	Container string `json:"c"`
	Observer  string `json:"o"`
	Pet       string `json:"p"`
	Slot      int    `json:"n"`
}

func encodeItemSearchCursor(value itemSearchCursor) string {
	data, _ := json.Marshal(value)
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeItemSearchCursor(encoded string) (itemSearchCursor, error) {
	var value itemSearchCursor
	if encoded == "" {
		return value, nil
	}
	if len(encoded) > 512 {
		return value, ErrInvalid
	}
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || json.Unmarshal(data, &value) != nil ||
		len(value.Server) == 0 || len(value.Server) > 100 ||
		len(value.Container) == 0 || len(value.Container) > 64 ||
		len(value.Observer) != 36 || len(value.Pet) > 64 ||
		value.Slot < 0 || value.Slot > 65535 {
		return itemSearchCursor{}, ErrInvalid
	}
	return value, nil
}

// ItemSearch reads only the canonical, currently observed resource-item rows.
// Shared guild storage selects one complete observer revision per server/guild
// before joining slots, so two observers never multiply guild-owned items.
func (s *Store) ItemSearch(ctx context.Context, filter ItemSearchFilter) (ItemSearchPage, error) {
	if s == nil || s.pool == nil {
		return ItemSearchPage{}, ErrInvalid
	}
	if len(filter.Server) > 100 || len(filter.Query) > 128 || len(filter.Container) > 64 ||
		len(filter.Owner) > 100 || len(filter.Type) > 64 || len(filter.Subcategory) > 64 ||
		len(filter.Degree) > 2 || (filter.Degree != "" && (filter.Degree[0] < '1' || filter.Degree[0] > '9')) {
		return ItemSearchPage{}, ErrInvalid
	}
	if filter.Container != "" && !isItemResource(filter.Container) {
		return ItemSearchPage{}, ErrInvalid
	}
	if filter.Degree != "" {
		n, err := strconv.Atoi(filter.Degree)
		if err != nil || n < 1 || n > 20 {
			return ItemSearchPage{}, ErrInvalid
		}
	}
	if filter.Limit < 1 || filter.Limit > 100 {
		return ItemSearchPage{}, ErrInvalid
	}
	cursor, err := decodeItemSearchCursor(filter.Cursor)
	if err != nil {
		return ItemSearchPage{}, err
	}
	useTaxonomy := filter.Type != "" || filter.Subcategory != "" || filter.Degree != ""
	if useTaxonomy && filter.Server == "" {
		// Model IDs can have different definitions on different servers.
		return ItemSearchPage{}, errors.New("item taxonomy filters require a server")
	}
	models := make([]int64, 0)
	if useTaxonomy {
		dataset := ""
		if s.metadata != nil {
			dataset = s.metadata.Servers[strings.ToLower(strings.TrimSpace(filter.Server))]
		}
		if s.metadata != nil {
			for modelKey, definition := range s.metadata.Catalogs[dataset].Items {
				pres := definition.Presentation
				kind, _ := pres["sort_type"].(string)
				sub, _ := pres["subcategory"].(string)
				degree := fmt.Sprint(pres["degree"])
				if filter.Type != "" && !strings.EqualFold(kind, filter.Type) {
					continue
				}
				if filter.Subcategory != "" && !strings.EqualFold(sub, filter.Subcategory) {
					continue
				}
				if filter.Degree != "" && degree != filter.Degree {
					continue
				}
				if value, err := strconv.ParseInt(modelKey, 10, 64); err == nil && value >= 0 {
					models = append(models, value)
				}
			}
		}
		if len(models) == 0 {
			return ItemSearchPage{Items: []ItemSearchEntry{}}, nil
		}
	}
	// resource observation and slot revision are joined atomically; a closed
	// storage does not retain queryable 'current' slots.
	const guildCTE = `WITH guild_snapshot AS (
		SELECT DISTINCT ON (server_key,guild_key)
			server_key,guild_key,observer_character_id,revision,session_id
		FROM character_resource_observations
		WHERE resource_key='guild_storage' AND guild_key<>'' AND availability='observed'
		ORDER BY server_key,guild_key,observed_at DESC NULLS LAST,updated_at DESC,observer_character_id
	)`
	const scope = `FROM character_resource_items i
	JOIN characters c ON c.character_id=i.observer_character_id
	JOIN character_resource_observations o
		ON o.observer_character_id=i.observer_character_id
		AND o.resource_key=i.container_key AND o.revision=i.revision
		AND o.session_id=i.session_id AND o.availability='observed'
	LEFT JOIN guild_snapshot g ON g.server_key=i.server_key AND g.guild_key=i.guild_key
	WHERE (i.container_key<>'guild_storage' OR
		(i.guild_key<>'' AND g.observer_character_id=i.observer_character_id
			AND g.revision=i.revision AND g.session_id=i.session_id))
	AND ($1='' OR i.server_key=lower($1))
	AND ($2='' OR coalesce(i.name,'') ILIKE '%'||$2||'%' OR
		coalesce(i.server_name,'') ILIKE '%'||$2||'%' OR coalesce(i.model::text,'') ILIKE '%'||$2||'%')
	AND ($3='' OR i.container_key=$3)
	AND ($4='' OR (i.container_key='guild_storage' AND i.guild_key=lower($4))
		OR (i.container_key<>'guild_storage' AND
			(lower(c.character_name)=lower($4) OR i.owner_character_id::text=$4 OR i.pet_id=$4)))
	AND ($5::boolean OR i.container_key<>'equipment')
	AND (NOT $6::boolean OR i.model=ANY($7::bigint[]))`
	args := []any{filter.Server, filter.Query, filter.Container, filter.Owner,
		filter.IncludeEquipment, useTaxonomy, models}
	// Counting and paging from different snapshots can mix two guild
	// observers' replacements, producing a total that doesn't match the page.
	// Use one read-only snapshot for the complete response.
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return ItemSearchPage{}, fmt.Errorf("begin item search: %w", err)
	}
	defer tx.Rollback(ctx)
	var total int64
	if err := tx.QueryRow(ctx, guildCTE+` SELECT count(*) `+scope, args...).Scan(&total); err != nil {
		return ItemSearchPage{}, fmt.Errorf("count item search: %w", err)
	}
	page := ItemSearchPage{Items: make([]ItemSearchEntry, 0, filter.Limit), Total: total}
	rows, err := tx.Query(ctx, guildCTE+`
	SELECT i.observer_character_id::text,i.server_key,i.container_key,i.pet_id,
		i.source_slot,i.guild_key,i.revision,i.observed_at,o.updated_at,
		i.raw_item,coalesce(i.model,0),coalesce(i.server_name,''),coalesce(i.quantity::text,''),
		c.character_name,c.server_name
	`+scope+`
	AND ($8='' OR (i.server_key,i.container_key,i.observer_character_id::text,i.pet_id,i.source_slot) >
		($8::text,$9::text,$10::text,$11::text,$12::integer))
	ORDER BY i.server_key,i.container_key,i.observer_character_id::text,i.pet_id,i.source_slot
	LIMIT $13`,
		append(args, cursor.Server, cursor.Container, cursor.Observer, cursor.Pet, cursor.Slot, filter.Limit+1)...)

	if err != nil {
		return ItemSearchPage{}, fmt.Errorf("query item search: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var entry ItemSearchEntry
		var serverKey, code string
		var model int64
		var raw []byte
		if err := rows.Scan(&entry.ObserverCharacterID, &serverKey, &entry.Container,
			&entry.PetID, &entry.Slot, &entry.Guild, &entry.Revision, &entry.ObservedAt,
			&entry.CheckedAt, &raw, &model, &code, &entry.QuantityExact, &entry.ObserverName, &entry.Server); err != nil {
			return ItemSearchPage{}, err
		}
		entry.serverKey = serverKey
		if err := json.Unmarshal(raw, &entry.Item); err != nil {
			return ItemSearchPage{}, err
		}
		if entry.Container == "guild_storage" {
			entry.OwnerKind = "guild"
			entry.OwnerID = entry.Guild
			entry.OwnerName = entry.Guild
		} else if entry.Container == "pets" {
			entry.OwnerKind = "pet"
			entry.OwnerID = entry.PetID
		} else {
			entry.OwnerKind = "character"
			entry.OwnerID = entry.ObserverCharacterID
			entry.OwnerName = entry.ObserverName
		}
		var modelPtr *int64
		if model > 0 {
			modelPtr = &model
		}
		entry.Metadata = s.ItemPresentation(entry.Server, modelPtr, code)
		entry.Item = s.EnrichItemRecord(entry.Server, entry.Item)
		page.Items = append(page.Items, entry)
	}
	if err := rows.Err(); err != nil {
		return ItemSearchPage{}, err
	}
	if len(page.Items) > filter.Limit {
		page.Items = page.Items[:filter.Limit]
		last := page.Items[len(page.Items)-1]
		page.NextCursor = encodeItemSearchCursor(itemSearchCursor{
			Server: last.serverKey, Container: last.Container,
			Observer: last.ObserverCharacterID, Pet: last.PetID, Slot: last.Slot,
		})
	}
	rows.Close()
	if err := tx.Commit(ctx); err != nil {
		return ItemSearchPage{}, fmt.Errorf("commit item search snapshot: %w", err)
	}
	return page, nil
}
