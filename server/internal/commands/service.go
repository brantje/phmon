package commands

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	agentdomain "phmon/server/internal/agents"
	"phmon/server/internal/chat"
)

const (
	operatorLimitPerMinute  = 60
	characterLimitPerMinute = 10
	maxRateBuckets          = 4096
	commandTTL              = 10 * time.Second
)

type SubmitInput struct {
	CharacterID       string
	ExpectedSessionID string
	Name              string
	Args              json.RawMessage
	IdempotencyKey    string
	Confirmation      bool
}

type rateBucket struct {
	start time.Time
	count int
}

type Service struct {
	store            *Store
	capabilities     CapabilityChecker
	navigation       NavigationAdmission
	dispatcher       *Dispatcher
	reverseReturn    ReverseReturnContextProvider
	reverseLocations ReverseReturnLocationProvider
	now              func() time.Time

	mu             sync.Mutex
	operatorRates  map[string]rateBucket
	characterRates map[string]rateBucket
}

func (s *Service) SetReverseReturnContext(provider ReverseReturnContextProvider) {
	s.reverseReturn = provider
}

func (s *Service) SetDispatcher(dispatcher *Dispatcher) { s.dispatcher = dispatcher }
func (s *Service) SetNavigationAdmission(admission NavigationAdmission) {
	s.navigation = admission
}
func (s *Service) DispatchNow() {
	if s.dispatcher != nil {
		s.dispatcher.Notify()
	}
}
func (s *Service) Acknowledge(ctx context.Context, id, agentID, sessionID string, generation uint64, at time.Time) (bool, error) {
	return s.store.Acknowledge(ctx, id, agentID, sessionID, generation, at)
}
func (s *Service) Result(ctx context.Context, id, agentID, sessionID string, generation uint64, at time.Time, result ResultInput) (bool, error) {
	return s.store.RecordResult(ctx, id, agentID, sessionID, generation, at, result)
}
func (s *Service) History(ctx context.Context, characterID, name, state string, limit int) ([]Command, error) {
	return s.store.ListHistory(ctx, characterID, name, state, limit)
}

func (s *Service) GetByID(ctx context.Context, id string) (Command, error) {
	return s.store.GetByID(ctx, id)
}
func (s *Service) CommandsByIdempotencyKeys(ctx context.Context, operatorIdentity string, keys []string) ([]Command, error) {
	return s.store.ListByIdempotencyKeys(ctx, operatorIdentity, keys)
}
func (s *Service) ResolveTarget(ctx context.Context, characterID string) (Target, error) {
	return s.store.ResolveTarget(ctx, characterID)
}
func (s *Service) SaveControlState(ctx context.Context, characterID, sessionID, agentID string, generation uint64, state ControlState) error {
	target, err := s.store.ResolveTarget(ctx, characterID)
	if err != nil {
		return err
	}
	if target.SessionID != sessionID || target.AgentID != agentID || target.Generation != generation {
		return ErrStaleSession
	}
	return s.store.SaveControlState(ctx, characterID, sessionID, state)
}
func (s *Service) CurrentTrainingAreas(ctx context.Context, server string, limit int) ([]TrainingAreaObservation, error) {
	return s.store.CurrentTrainingAreas(ctx, server, limit)
}
func (s *Service) Controls(ctx context.Context, characterID string) (map[string]any, error) {
	target, err := s.store.ResolveTarget(ctx, characterID)
	if errors.Is(err, ErrStaleSession) || errors.Is(err, ErrNotFound) {
		return map[string]any{"character_id": characterID, "session_id": "", "capabilities": map[string]Capability{}, "training": nil}, nil
	}
	if err != nil {
		return nil, err
	}
	state, err := s.store.CurrentControlState(ctx, characterID)
	if err != nil {
		return nil, err
	}
	snapshot := s.controlSnapshot(target, state)
	s.addReverseReturnContexts(ctx, []map[string]any{snapshot})
	return snapshot, nil
}

func (s *Service) ControlsForTargets(ctx context.Context, characterIDs []string) ([]map[string]any, error) {
	targets, err := s.store.CurrentControlTargets(ctx, characterIDs)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(characterIDs))
	for _, characterID := range characterIDs {
		target, ok := targets[characterID]
		if !ok {
			result = append(result, emptyControlSnapshot(characterID))
			continue
		}
		if target.SessionID == "" || target.AgentID == "" || target.Generation == 0 {
			result = append(result, emptyControlSnapshot(characterID))
			continue
		}
		training := target.Training
		if training != nil {
			training.SessionID = target.SessionID
		}
		result = append(result, s.controlSnapshot(
			Target{CharacterID: target.CharacterID, Server: target.Server, SessionID: target.SessionID, AgentID: target.AgentID, Generation: target.Generation, Region: target.Region},
			training,
		))
	}
	s.addReverseReturnContexts(ctx, result)
	return result, nil
}

func emptyControlSnapshot(characterID string) map[string]any {
	return map[string]any{"character_id": characterID, "session_id": "", "agent_protocol_version": 0,
		"capabilities": map[string]Capability{}, "training": nil}
}

func (s *Service) controlSnapshot(target Target, state *ControlState) map[string]any {
	protocol := 0
	if versioned, ok := s.capabilities.(interface {
		ProtocolVersion(agentID string, generation uint64) int
	}); ok {
		protocol = versioned.ProtocolVersion(target.AgentID, target.Generation)
	}
	return map[string]any{"character_id": target.CharacterID, "session_id": target.SessionID,
		"capabilities": s.commandCapabilities(target), "training": state, "agent_protocol_version": protocol,
		"reverse_return_named_locations": s.reverseReturnLocations(target.Server)}
}

func (s *Service) commandCapabilities(target Target) map[string]Capability {
	capabilities := make(map[string]Capability)
	for _, name := range []string{"bot.start", "bot.stop", "trace.start", "trace.stop", "training.area.set", "training.radius.set", "character.walk", "character.navigate", "character.navigate.stop", "character.teleport", "character.recall_point.designate", "character.return", "character.reverse_return", "character.disconnect", "client.clientless", "chat.send"} {
		ok, reason := false, "plugin_upgrade_required"
		if s.capabilities != nil {
			ok, reason = s.capabilities.CommandSupport(target.AgentID, target.Generation, name)
		}
		capability := Capability{Supported: ok, Reason: reason}
		if name == "training.area.set" && ok {
			if checker, exists := s.capabilities.(interface {
				CommandModeSupport(string, uint64, string, string) (bool, string)
			}); exists {
				for _, mode := range []string{"current_position", "position", "named"} {
					if supported, _ := checker.CommandModeSupport(target.AgentID, target.Generation, name, mode); supported {
						capability.Modes = append(capability.Modes, mode)
					}
				}
				capability.Supported = len(capability.Modes) > 0
			}
		}
		if name == "character.reverse_return" && ok {
			if checker, exists := s.capabilities.(interface {
				CommandModeSupport(string, uint64, string, string) (bool, string)
			}); exists {
				for _, mode := range []string{"last_return", "last_death", "party_member", "named_location"} {
					if supported, _ := checker.CommandModeSupport(target.AgentID, target.Generation, name, mode); supported {
						capability.Modes = append(capability.Modes, mode)
					}
				}
				capability.Supported = len(capability.Modes) > 0
			}
		}
		if name == "chat.send" && ok {
			if checker, exists := s.capabilities.(interface {
				CommandModeSupport(string, uint64, string, string) (bool, string)
			}); exists {
				for _, mode := range []string{"general", "private", "party", "guild", "union", "global"} {
					if supported, _ := checker.CommandModeSupport(target.AgentID, target.Generation, name, mode); supported {
						capability.Modes = append(capability.Modes, mode)
					}
				}
				capability.Supported = len(capability.Modes) > 0
			}
		}
		capabilities[name] = capability
	}
	return capabilities
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
	// Position mode carries an explicit region, including signed cave IDs;
	// phBot validates whether the destination was accepted.

	if validated.Name == "character.reverse_return" {
		var args reverseReturnArgs
		_ = json.Unmarshal(validated.Args, &args)
		if *args.Type == 3 {
			if reason := s.namedReverseReturnReason(target.Server, args.Name); reason != "" {
				return Command{}, false, reason, ErrUnsupported
			}
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
	if mode, ok := commandMode(validated); ok {
		if modeChecker, ok := s.capabilities.(interface {
			CommandModeSupport(string, uint64, string, string) (bool, string)
		}); ok {
			supported, reason = modeChecker.CommandModeSupport(target.AgentID, target.Generation, validated.Name, mode)
			if !supported {
				if reason == "" {
					reason = "unsupported_argument_mode"
				}
				return Command{}, false, reason, ErrUnsupported
			}
		}
	}
	if validated.Name == "character.navigate.stop" {
		var args navigateStopArgs
		if json.Unmarshal(validated.Args, &args) != nil {
			return Command{}, false, "", ErrInvalid
		}
		if s.navigation == nil {
			return Command{}, false, "navigation_unavailable", ErrUnsupported
		}
		ok, reason := s.navigation.CanStopNavigation(input.CharacterID, input.ExpectedSessionID, args.CommandID, args.RouteSequence)
		if !ok {
			if reason == "" {
				reason = "route_not_active"
			}
			return Command{}, false, reason, ErrUnsupported
		}
	}
	if validated.Name == "chat.send" {
		var args struct {
			Channel string `json:"channel"`
		}
		if json.Unmarshal(validated.Args, &args) != nil || !chat.ValidChannel(args.Channel) {
			return Command{}, false, "", ErrInvalid
		}
		if modeChecker, ok := s.capabilities.(interface {
			CommandModeSupport(string, uint64, string, string) (bool, string)
		}); ok {
			supported, modeReason := modeChecker.CommandModeSupport(target.AgentID, target.Generation, validated.Name, args.Channel)
			if !supported {
				if modeReason == "" {
					modeReason = "unsupported_channel"
				}
				return Command{}, false, modeReason, ErrUnsupported
			}
		}
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
	if err == nil && !duplicate {
		s.DispatchNow()
	}
	return command, duplicate, "", err
}

func commandMode(validated Validated) (string, bool) {
	if validated.Name == "character.reverse_return" {
		var args reverseReturnArgs
		if json.Unmarshal(validated.Args, &args) != nil || args.Type == nil || *args.Type < 0 || *args.Type > 3 {
			return "", false
		}
		return []string{"last_return", "last_death", "party_member", "named_location"}[*args.Type], true
	}
	if validated.Name != "training.area.set" {
		return "", false
	}
	var args struct {
		Mode string `json:"mode"`
	}
	if json.Unmarshal(validated.Args, &args) != nil {
		return "", false
	}
	return args.Mode, args.Mode != ""
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
