package analytics

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

const CharacterStorageGoldSampleIntervalSeconds = 60

// RecordCharacterStorageGold stores a personal-bank observation for one character.
func RecordCharacterStorageGold(ctx context.Context, tx pgx.Tx, characterID, sessionID, agentID string, generation, revision uint64, gold int64) (bool, error) {
	if tx == nil || characterID == "" || sessionID == "" || agentID == "" || generation == 0 || revision == 0 || gold < 0 {
		return false, errors.New("invalid character storage gold observation")
	}
	tag, err := tx.Exec(ctx, `
INSERT INTO character_storage_gold_samples (
    character_id,session_id,resource_revision,sampled_at,gold
)
SELECT c.character_id,cs.session_id,$5,clock_timestamp(),$6
FROM characters c
JOIN character_sessions cs ON cs.character_id=c.character_id
WHERE c.character_id=$1
  AND cs.session_id=$2::uuid
  AND cs.agent_id=$3::uuid
  AND cs.connection_generation=$4
  AND cs.ended_at IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM (
          SELECT previous.sampled_at,previous.gold
          FROM character_storage_gold_samples previous
          WHERE previous.character_id=c.character_id
            AND previous.session_id=cs.session_id
          ORDER BY previous.sampled_at DESC,previous.sample_id DESC
          LIMIT 1
      ) previous
      WHERE previous.gold=$6
        AND previous.sampled_at > clock_timestamp() - make_interval(secs => $7::double precision)
  )
RETURNING sample_id`, characterID, sessionID, agentID, generation, revision, gold, CharacterStorageGoldSampleIntervalSeconds)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
