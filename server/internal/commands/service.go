package commands

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	agentdomain "phmon/server/internal/agents"
)

const (
	operatorLimitPerMinute  = 60
	characterLimitPerMinute = 10
	maxRateBuckets          = 4096
	commandTTL              = 10 * time.Second
)

type SubmitInput struct {
	CharacterID      string
	ExpectedSessionID string
	Name             string
	Args             json.RawMessage
	IdempotencyKey   string
	Confirmation     bool
}

type rateBucket struct {
	start time.Time
	count int
}

type Service struct {
	store        *Store
	capabilities CapabilityChecker
	now          func() time.Time

	mu            sync.Mutex
	operatorRates map[string]rateBucket
	characterRates map[string]rateBucket
}

func NewService(store *Store, capabilities CapabilityChecker) *Service {
	return &Service{
		store:          store,
		capabilities:   capabilities,
		now:            func() time.Time { return time.Now().UTC() },
		operatorRates:  make(map[string]rateBucket),
		characterRates: make(map[string]rateBucket),
	}
}

func (s *Service) Submit(ctx context.Context, operatorIdentity string, input SubmitInput) (Command, bool, string, error) {
	operatorIdentity = strings.TrimSpace(operatorIdentity)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if operatorIdentity == "" || len(operatorIdentity) > 64 ||
		!agentdomain.ValidAgentID(input.CharacterID) ||
		!agentdomain.ValidAgentID(input.ExpectedSessionID) ||
		len(input.IdempotencyKey) < 1 || len(input.IdempotencyKey) > 128 ||
		strings.ContainsRune(input.IdempotencyKey, 0) {
		return Command{}, false, "", ErrInvalid
	}

	validated, err := Validate(input.Name, input.Args, input.Confirmation)
	if err != nil {
		return Command{}, false, "", err
	}
	requestHash := hashRequest(input.CharacterID, input.ExpectedSessionID, input.IdempotencyKey, validated)
	if existing, existingHash, found, err := s.store.FindByIdempotency(ctx, operatorIdentity, input.IdempotencyKey); err != nil {
		return Command{}, false, "", err
	} else if found {
		if existingHash != requestHash {
			return Command{}, false, "", ErrIdempotencyConflict
		}
		return existing, true, "", nil
	}

	now := s.now()
	if !s.allowRate(operatorIdentity, input.CharacterID, now) {
		return Command{}, false, "", ErrRateLimited
	}

	target, err := s.store.ResolveTarget(ctx, input.CharacterID)
	if err != nil {
		return Command{}, false, "", err
	}
	if target.SessionID != input.ExpectedSessionID {
		return Command{}, false, "", ErrStaleSession
	}

	if validated.Name == "character.walk" {
		var args walkArgs
		if err := json.Unmarshal(validated.Args, &args); err != nil {
			return Command{}, false, "", ErrInvalid
		}
		if target.Region == nil || args.Region != *target.Region {
			return Command{}, false, "", ErrInvalid
		}
	}

	supported, reason := false, "plugin_upgrade_required"
	if s.capabilities != nil {
		supported, reason = s.capabilities.CommandSupport(target.AgentID, target.Generation, validated.Name)
	}
	if !supported {
		if reason == "" {
			reason = "unsupported"
		}
		return Command{}, false, reason, ErrUnsupported
	}

	admission := Admission{
		OperatorIdentity:  operatorIdentity,
		IdempotencyKey:    input.IdempotencyKey,
		RequestHash:       requestHash,
		ExpectedSessionID: input.ExpectedSessionID,
		Validated:         validated,
		ExpiresAt:         now.Add(commandTTL),
	}
	command, duplicate, err := s.store.Admit(ctx, target, admission)
	return command, duplicate, "", err
}

func hashRequest(characterID, sessionID, idempotencyKey string, validated Validated) [32]byte {
	payload := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%t", characterID, sessionID, idempotencyKey, validated.Name, validated.Args, validated.Confirmation)
	return sha256.Sum256([]byte(payload))
}

func (s *Service) allowRate(operatorIdentity, characterID string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return allowBucket(s.operatorRates, operatorIdentity, operatorLimitPerMinute, now) &&
		allowBucket(s.characterRates, characterID, characterLimitPerMinute, now)
}

func allowBucket(buckets map[string]rateBucket, key string, limit int, now time.Time) bool {
	if len(buckets) >= maxRateBuckets {
		for candidate, bucket := range buckets {
			if now.Sub(bucket.start) >= time.Minute {
				delete(buckets, candidate)
			}
		}
		if len(buckets) >= maxRateBuckets {
			return false
		}
	}
	bucket := buckets[key]
	if bucket.start.IsZero() || now.Sub(bucket.start) >= time.Minute {
		bucket = rateBucket{start: now}
	}
	if bucket.count >= limit {
		return false
	}
	bucket.count++
	buckets[key] = bucket
	return true
}
