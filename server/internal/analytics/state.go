package analytics

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

const CharacterSampleIntervalSeconds = 10

// CharacterState is the accepted primitive state needed for historical rates.
// Pointer fields preserve unavailable values rather than converting them to 0.
type CharacterState struct {
	Level     *int
	CurrentXP *int64
	MaxXP     *int64
	SP        *int64
	Gold      *int64
	Region    *int
	Zone      *string
	Botting   *bool
	Dead      *bool
}

// RecordCharacterState writes an observation in the caller's accepted state
// transaction. The caller already holds the character/session admission lock.
// Numeric changes are sampled at most once per interval; level, XP requirement,
// region, death and botting transitions are captured promptly.
func RecordCharacterState(ctx context.Context, tx pgx.Tx, characterID, sessionID, agentID string, generation uint64, state CharacterState) (bool, error) {
	if tx == nil || characterID == "" || sessionID == "" || agentID == "" || generation == 0 {
		return false, errors.New("invalid character analytics sample identity")
	}
	tag, err := tx.Exec(ctx, `
INSERT INTO character_metric_samples (
    character_id,session_id,agent_id,server_key,sampled_at,level,current_exp,max_exp,sp,gold,
    region,zone_name,botting,dead
)
SELECT c.character_id,cs.session_id,cs.agent_id,c.server_key,clock_timestamp(),
       $5,$6,$7,$8,$9,$10,$11,$12,$13
FROM characters c
JOIN character_sessions cs ON cs.character_id=c.character_id
WHERE c.character_id=$1
  AND cs.session_id=$2::uuid
  AND cs.agent_id=$3::uuid
  AND cs.connection_generation=$4
  AND cs.ended_at IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM (
          SELECT previous.sampled_at,previous.level,previous.max_exp,previous.region,
                 previous.zone_name,previous.botting,previous.dead
          FROM character_metric_samples previous
          WHERE previous.session_id=cs.session_id
          ORDER BY previous.sampled_at DESC,previous.sample_id DESC
          LIMIT 1
      ) previous
      WHERE previous.sampled_at > clock_timestamp() - make_interval(secs => $14::double precision)
        AND ROW(previous.level,previous.max_exp,previous.region,previous.zone_name,previous.botting,previous.dead)
            IS NOT DISTINCT FROM ROW($5::integer,$7::bigint,$10::integer,$11::text,$12::boolean,$13::boolean)
  )
RETURNING sample_id`, characterID, sessionID, agentID, generation,
		state.Level, state.CurrentXP, state.MaxXP, state.SP, state.Gold,
		state.Region, state.Zone, state.Botting, state.Dead, CharacterSampleIntervalSeconds)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
