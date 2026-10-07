package analytics

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

const GuildGoldSampleIntervalSeconds = 60

// RecordGuildGold stores observations per observer. Query code reconciles
// observers for the same guild and never adds their repeated balances together.
func RecordGuildGold(ctx context.Context, tx pgx.Tx, server, guild, characterID, sessionID, agentID string, generation, revision uint64, gold int64) (bool, error) {
	server = strings.ToLower(strings.TrimSpace(server))
	guild = strings.ToLower(strings.TrimSpace(guild))
	if tx == nil || server == "" || len(server) > 100 || guild == "" || len(guild) > 100 ||
		characterID == "" || sessionID == "" || agentID == "" || generation == 0 || revision == 0 || gold < 0 {
		return false, errors.New("invalid guild gold observation")
	}
	tag, err := tx.Exec(ctx, `
INSERT INTO guild_gold_samples (
    server_key,guild_key,observer_character_id,session_id,resource_revision,sampled_at,gold
)
SELECT $1,$2,c.character_id,cs.session_id,$7,clock_timestamp(),$8
FROM characters c
JOIN character_sessions cs ON cs.character_id=c.character_id
WHERE c.character_id=$3
  AND c.server_key=$1
  AND lower(btrim(c.guild_name))=$2
  AND cs.session_id=$4::uuid
  AND cs.agent_id=$5::uuid
  AND cs.connection_generation=$6
  AND cs.ended_at IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM (
          SELECT previous.sampled_at,previous.gold
          FROM guild_gold_samples previous
          WHERE previous.observer_character_id=c.character_id
            AND previous.session_id=cs.session_id
            AND previous.server_key=$1
            AND previous.guild_key=$2
          ORDER BY previous.sampled_at DESC,previous.sample_id DESC
          LIMIT 1
      ) previous
      WHERE previous.gold=$8
        AND previous.sampled_at > clock_timestamp() - make_interval(secs => $9::double precision)
  )
RETURNING sample_id`, server, guild, characterID, sessionID, agentID,
		generation, revision, gold, GuildGoldSampleIntervalSeconds)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
