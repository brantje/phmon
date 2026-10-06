// Package navigation keeps generated phBot routes in memory for map display.
// Route geometry is deliberately transient and never becomes position history.
package navigation

import (
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"phmon/server/internal/mapprofile"
)

const (
	SchemaVersion           = 1
	MaxInstructions         = 4096
	MaxObservedInstructions = 4096
	MaxFrameBytes           = 512 * 1024
	MaxObservedFrameBytes   = 512 * 1024
	ArrivalRadius           = 12.0
	MaxLiveBytes            = 384 * 1024
)

var ErrInvalid = errors.New("invalid navigation route")

type Position struct {
	Region int       `json:"region"`
	X      float64   `json:"x"`
	Y      float64   `json:"y"`
	Z      *float64  `json:"z,omitempty"`
	At     time.Time `json:"observed_at"`
}

type Instruction struct {
	Index      int     `json:"index"`
	Kind       string  `json:"kind"`
	Region     *int    `json:"region,omitempty"`
	X          float64 `json:"x,omitempty"`
	Y          float64 `json:"y,omitempty"`
	Z          float64 `json:"z,omitempty"`
	DurationMS int64   `json:"duration_ms,omitempty"`
}

type Input struct {
	SchemaVersion int           `json:"schema_version"`
	CommandID     string        `json:"command_id"`
	CharacterID   string        `json:"character_id"`
	SessionID     string        `json:"session_id"`
	Sequence      uint64        `json:"route_sequence"`
	InvokedAt     time.Time     `json:"invoked_at"`
	Source        *Position     `json:"source,omitempty"`
	Instructions  []Instruction `json:"instructions"`
}

// ObservedInput is a display-only route submitted by another plugin. It has no
// PhMon command and must not be treated as a navigation the server started.
type ObservedInput struct {
	SchemaVersion int           `json:"schema_version"`
	CharacterID   string        `json:"character_id"`
	SessionID     string        `json:"session_id"`
	Sequence      uint64        `json:"route_sequence"`
	Active        bool          `json:"active"`
	InvokedAt     time.Time     `json:"invoked_at"`
	Source        *Position     `json:"source,omitempty"`
	Instructions  []Instruction `json:"instructions,omitempty"`
}

type Point struct {
	Region int     `json:"region"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Z      float64 `json:"z"`
}

type Block struct {
	AreaID  string  `json:"area_id"`
	FloorID string  `json:"floor_id"`
	Points  []Point `json:"points"`
}

type View struct {
	CommandID             string    `json:"command_id"`
	CharacterID           string    `json:"character_id"`
	SessionID             string    `json:"session_id"`
	Sequence              uint64    `json:"route_sequence"`
	Server                string    `json:"server"`
	DatasetID             string    `json:"dataset_id"`
	DatasetVersion        string    `json:"dataset_version"`
	AreaID                string    `json:"area_id,omitempty"`
	FloorID               string    `json:"floor_id,omitempty"`
	Destination           Point     `json:"destination"`
	DestinationAreaID     string    `json:"destination_area_id,omitempty"`
	DestinationFloorID    string    `json:"destination_floor_id,omitempty"`
	CurrentAnchor         *Point    `json:"current_anchor,omitempty"`
	Status                string    `json:"status"`
	Reason                string    `json:"reason,omitempty"`
	UpdatedAt             time.Time `json:"updated_at"`
	Blocks                []Block   `json:"blocks"`
	Arrived               bool      `json:"arrived,omitempty"`
	GeometryOmitted       bool      `json:"geometry_omitted,omitempty"`
	InstructionCount      int       `json:"instruction_count,omitempty"`
	CompletedInstructions int       `json:"completed_instructions,omitempty"`
	Progress              *float64  `json:"progress,omitempty"`
	ETASeconds            *int      `json:"eta_seconds,omitempty"`
}

type route struct {
	Input
	agentID      string
	generation   uint64
	server       string
	datasetID    string
	destination  Point
	cursor       int
	status       string
	reason       string
	updatedAt    time.Time
	arrived      bool
	lastPosition *Position
	anchor       *Position
	stopped      bool
	recentMoves  []Position
}

type routeOwner struct {
	agentID    string
	generation uint64
}

type Store struct {
	mu              sync.RWMutex
	routes          map[string]route // one latest command route, including terminal sequence, per session
	pending         map[string]routeOwner
	observed        map[string]route // display-only routes from another plugin
	observedFloor   map[string]uint64
	observedPending map[string]routeOwner
	lifecycle       [128]sync.Mutex
}

func NewStore() *Store {
	return &Store{
		routes:          make(map[string]route),
		pending:         make(map[string]routeOwner),
		observed:        make(map[string]route),
		observedFloor:   make(map[string]uint64),
		observedPending: make(map[string]routeOwner),
	}
}

func Valid(input Input) error {
	if input.CommandID == "" || input.CharacterID == "" || input.SessionID == "" || input.Sequence == 0 ||
		input.SchemaVersion != SchemaVersion || input.InvokedAt.IsZero() || len(input.Instructions) == 0 || len(input.Instructions) > MaxInstructions {
		return ErrInvalid
	}
	if err := validInstructions(input.Instructions, MaxInstructions, false); err != nil {
		return err
	}
	if input.Source != nil && (!validRegion(input.Source.Region) || !coordinate(input.Source.X) || !coordinate(input.Source.Y) ||
		input.Source.Z != nil && !coordinate(*input.Source.Z) || input.Source.At.IsZero()) {
		return ErrInvalid
	}
	return nil
}

func ValidObserved(input ObservedInput) error {
	if input.CharacterID == "" || input.SessionID == "" || input.Sequence == 0 ||
		input.SchemaVersion != SchemaVersion || input.InvokedAt.IsZero() {
		return ErrInvalid
	}
	if !input.Active {
		if len(input.Instructions) != 0 || input.Source != nil {
			return ErrInvalid
		}
		return nil
	}
	if input.Source == nil || len(input.Instructions) == 0 || len(input.Instructions) > MaxObservedInstructions {
		return ErrInvalid
	}
	if err := validInstructions(input.Instructions, MaxObservedInstructions, true); err != nil {
		return err
	}
	if !validRegion(input.Source.Region) || !coordinate(input.Source.X) || !coordinate(input.Source.Y) ||
		input.Source.Z == nil || !coordinate(*input.Source.Z) || input.Source.At.IsZero() {
		return ErrInvalid
	}
	for _, step := range input.Instructions {
		if step.Kind == "walk" {
			return nil
		}
	}
	return ErrInvalid
}

func validInstructions(steps []Instruction, maxCount int, allowWalkRegion bool) error {
	lastIndex := -1
	for _, step := range steps {
		if step.Index <= lastIndex || step.Index >= maxCount {
			return ErrInvalid
		}
		lastIndex = step.Index
		switch step.Kind {
		case "walk":
			if !coordinate(step.X) || !coordinate(step.Y) || !coordinate(step.Z) || step.DurationMS != 0 {
				return ErrInvalid
			}
			if step.Region != nil && (!allowWalkRegion || !validRegion(*step.Region)) {
				return ErrInvalid
			}
		case "wait":
			if step.Region != nil || step.DurationMS < 0 || step.DurationMS > 999999 || step.X != 0 || step.Y != 0 || step.Z != 0 {
				return ErrInvalid
			}
		case "teleport":
			if step.Region != nil || step.DurationMS != 0 || step.X != 0 || step.Y != 0 || step.Z != 0 {
				return ErrInvalid
			}
		default:
			return ErrInvalid
		}
	}
	return nil
}

func (s *Store) Replace(input Input, agentID string, generation uint64, server, dataset string, destination Point, now time.Time) bool {
	return s.ReplaceIf(input, agentID, generation, server, dataset, destination, now, nil)
}

// AlreadyApplied cheaply drops duplicate or obsolete presentation snapshots
// from the same authenticated owner. It never admits a new route or refreshes
// position freshness, and retains terminal sequences until session cleanup.
func (s *Store) AlreadyApplied(input Input, agentID string, generation uint64) bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	current, ok := s.routes[input.SessionID]
	return ok && current.CharacterID == input.CharacterID && current.agentID == agentID &&
		current.generation == generation && input.Sequence <= current.Sequence
}

// ReplaceIf stores a validated route only while its external owner fence is
// still current. The callback runs under that session's lifecycle lock so
// cleanup cannot race between the final ownership check and the route write.
func (s *Store) ReplaceIf(input Input, agentID string, generation uint64, server, dataset string, destination Point, now time.Time, ownerCurrent func() bool) bool {
	if s == nil || Valid(input) != nil || server == "" || dataset == "" || !validRegion(destination.Region) ||
		!coordinate(destination.X) || !coordinate(destination.Y) || !coordinate(destination.Z) {
		return false
	}
	if s.AlreadyApplied(input, agentID, generation) {
		return false
	}
	lifecycle := s.sessionLifecycleLock(input.SessionID)
	lifecycle.Lock()
	defer lifecycle.Unlock()
	s.mu.Lock()
	s.pending[input.SessionID] = routeOwner{agentID: agentID, generation: generation}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.pending, input.SessionID)
		s.mu.Unlock()
	}()
	if ownerCurrent != nil && !ownerCurrent() {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.routes[input.SessionID]
	if ok && current.stopped && input.Sequence == current.Sequence {
		return false
	}
	if ok && (input.Sequence <= current.Sequence || input.InvokedAt.Before(current.InvokedAt)) {
		return false
	}
	stored := route{Input: input, agentID: agentID, generation: generation, server: server, datasetID: dataset, destination: destination,
		status: "waiting_for_movement", updatedAt: now.UTC()}
	if input.Source != nil {
		copySource := *input.Source
		stored.anchor = &copySource
	}
	s.routes[input.SessionID] = stored
	return true
}

// ObservedDestination is the last walk point. Its region comes from the source
// cave context, or from the outdoor grid when the source is not inside a cave.
func ObservedDestination(server, dataset string, input ObservedInput) (Point, bool) {
	if input.Source == nil {
		return Point{}, false
	}
	var last *Instruction
	for index := range input.Instructions {
		if input.Instructions[index].Kind == "walk" {
			last = &input.Instructions[index]
		}
	}
	if last == nil {
		return Point{}, false
	}
	if last.Region != nil {
		return Point{Region: *last.Region, X: last.X, Y: last.Y, Z: last.Z}, true
	}
	profile, err := mapprofile.ForServer(server, dataset)
	if err != nil {
		return Point{}, false
	}
	if input.Source.Z != nil {
		if area, floor, ok := mapprofile.ClassifyCave(profile, &input.Source.Region, input.Source.Z); ok {
			if !(area == "job-temple" && floor != "1F") {
				return Point{Region: input.Source.Region, X: last.X, Y: last.Y, Z: last.Z}, true
			}
		}
	}
	if knownCaveRegion(profile, input.Source.Region) {
		return Point{}, false
	}
	region, ok := outdoorRegion(profile, last.X, last.Y)
	if !ok {
		return Point{}, false
	}
	return Point{Region: region, X: last.X, Y: last.Y, Z: last.Z}, true
}

// ReplaceObserved stores a display-only route without touching the command route
// for the same session. An equal or older sequence does not reset progress.
func (s *Store) ReplaceObserved(input ObservedInput, agentID string, generation uint64, server, dataset string, now time.Time, ownerCurrent func() bool) bool {
	if s == nil || !input.Active || ValidObserved(input) != nil || server == "" || dataset == "" {
		return false
	}
	destination, ok := ObservedDestination(server, dataset, input)
	if !ok {
		return false
	}
	lifecycle := s.sessionLifecycleLock(input.SessionID)
	lifecycle.Lock()
	defer lifecycle.Unlock()
	s.mu.Lock()
	s.observedPending[input.SessionID] = routeOwner{agentID: agentID, generation: generation}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.observedPending, input.SessionID)
		s.mu.Unlock()
	}()
	if ownerCurrent != nil && !ownerCurrent() {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.observedSequenceUsed(input.SessionID, input.Sequence) {
		return false
	}
	storedInput := Input{
		SchemaVersion: input.SchemaVersion, CharacterID: input.CharacterID, SessionID: input.SessionID,
		Sequence: input.Sequence, InvokedAt: input.InvokedAt, Source: input.Source, Instructions: input.Instructions,
	}
	stored := route{Input: storedInput, agentID: agentID, generation: generation, server: server, datasetID: dataset, destination: destination,
		status: "waiting_for_movement", updatedAt: now.UTC()}
	copySource := *input.Source
	stored.anchor = &copySource
	s.observed[input.SessionID] = stored
	return true
}

// ClearObserved removes the display-only route when the submitted sequence is
// still the current one. The sequence cannot be shown again.
func (s *Store) ClearObserved(input ObservedInput, agentID string, generation uint64, ownerCurrent func() bool) bool {
	if s == nil || input.Active || ValidObserved(input) != nil {
		return false
	}
	lifecycle := s.sessionLifecycleLock(input.SessionID)
	lifecycle.Lock()
	defer lifecycle.Unlock()
	if ownerCurrent != nil && !ownerCurrent() {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.observed[input.SessionID]
	if !ok || current.Sequence != input.Sequence || current.CharacterID != input.CharacterID ||
		current.agentID != agentID || current.generation != generation {
		return false
	}
	delete(s.observed, input.SessionID)
	if s.observedFloor[input.SessionID] < input.Sequence {
		s.observedFloor[input.SessionID] = input.Sequence
	}
	return true
}

func (s *Store) observedSequenceUsed(sessionID string, sequence uint64) bool {
	if current, ok := s.observed[sessionID]; ok && sequence <= current.Sequence {
		return true
	}
	floor, ok := s.observedFloor[sessionID]
	return ok && sequence <= floor
}

// Observe advances only from a fresh character state already accepted by the
// session-fenced character store. Repeated/out-of-order observations do nothing.
// Command routes and display-only observed routes advance independently.
func (s *Store) Observe(characterID, sessionID string, position Position) bool {
	if s == nil || characterID == "" || sessionID == "" || position.At.IsZero() || !validRegion(position.Region) ||
		!coordinate(position.X) || !coordinate(position.Y) || position.Z != nil && !coordinate(*position.Z) {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	reported := false
	for _, routes := range []map[string]route{s.routes, s.observed} {
		current, present := routes[sessionID]
		next, write, signal := advanceStoredRoute(current, present, characterID, position)
		if write {
			routes[sessionID] = next
		}
		if signal {
			reported = true
		}
	}
	return reported
}

func advanceStoredRoute(current route, present bool, characterID string, position Position) (route, bool, bool) {
	if !present || current.stopped || current.CharacterID != characterID || position.At.Before(current.InvokedAt) ||
		current.lastPosition != nil && !position.At.After(current.lastPosition.At) {
		return current, false, false
	}
	previous := current.lastPosition
	copyPosition := position
	current.lastPosition = &copyPosition
	current.updatedAt = position.At.UTC()
	if current.arrived {
		return current, true, false
	}
	if closeToDestination(current, position) {
		current.arrived = true
		current.status = "arrived"
		return current, true, true
	}

	// A barrier is crossed only by a fresh sample showing movement from the
	// previous block into the following scoped block. A timer never advances it.
	steps := current.Instructions
	if current.cursor < len(steps) && (steps[current.cursor].Kind == "wait" || steps[current.cursor].Kind == "teleport") {
		if crossBarrier(&current, position, previous) {
			current.status = "moving"
		} else {
			current.status = "transition_awaiting_evidence"
		}
		return current, true, true
	}
	progressed, uncertain := advanceWalk(&current, position)
	if uncertain {
		current.status = "progress_uncertain"
		current.reason = "position_not_near_remaining_route"
	} else if progressed {
		current.reason = ""
	}
	if current.cursor >= len(steps) && closeToDestination(current, position) {
		current.arrived = true
		current.status = "arrived"
	} else if uncertain {
		current.status = "progress_uncertain"
	} else if current.cursor >= len(steps) {
		current.status = "waiting_for_arrival"
	} else if steps[current.cursor].Kind == "wait" || steps[current.cursor].Kind == "teleport" {
		current.status = "transition_awaiting_evidence"
	} else if progressed || current.cursor > 0 {
		current.status = "moving"
	}
	if progressed && current.status == "moving" {
		appendRecentMove(&current, position)
	}
	return current, true, true
}

// CanStopNavigation reports whether a stop command may be admitted for the
// current in-memory route owned by the character session.
func (s *Store) CanStopNavigation(characterID, sessionID, commandID string, sequence uint64) (bool, string) {
	if s == nil || characterID == "" || sessionID == "" || commandID == "" || sequence == 0 {
		return false, "route_not_active"
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	current, ok := s.routes[sessionID]
	if !ok || current.stopped || current.arrived || current.CharacterID != characterID ||
		current.CommandID != commandID || current.Sequence != sequence {
		return false, "route_not_active"
	}
	switch current.status {
	case "moving", "waiting_for_arrival", "transition_awaiting_evidence", "progress_uncertain":
		return true, ""
	default:
		return false, "route_not_started"
	}
}

// MarkNavigationStopped records a terminal stop outcome for the active route.
func (s *Store) MarkNavigationStopped(sessionID, commandID string, sequence uint64, success bool, now time.Time) bool {
	if s == nil || sessionID == "" || commandID == "" || sequence == 0 {
		return false
	}
	lifecycle := s.sessionLifecycleLock(sessionID)
	lifecycle.Lock()
	defer lifecycle.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.routes[sessionID]
	if !ok || current.stopped || current.arrived || current.CommandID != commandID || current.Sequence != sequence {
		return false
	}
	current.stopped = true
	current.status = "stopped"
	if !success {
		current.status = "stop_failed"
		current.reason = "stop_script_failed"
	}
	current.updatedAt = now.UTC()
	current.recentMoves = nil
	s.routes[sessionID] = current
	return true
}

func (s *Store) RemoveSession(sessionID string) {
	if s != nil {
		lifecycle := s.sessionLifecycleLock(sessionID)
		lifecycle.Lock()
		defer lifecycle.Unlock()
		s.mu.Lock()
		delete(s.routes, sessionID)
		delete(s.pending, sessionID)
		delete(s.observed, sessionID)
		delete(s.observedFloor, sessionID)
		delete(s.observedPending, sessionID)
		s.mu.Unlock()
	}
}
func (s *Store) RemoveAgentSessions(sessionIDs []string) {
	for _, id := range sessionIDs {
		s.RemoveSession(id)
	}
}
func (s *Store) RemoveAgent(agentID string) {
	if s != nil {
		for _, id := range s.sessionsForOwner(agentID, nil) {
			s.RemoveSession(id)
		}
	}
}

func (s *Store) RemoveAgentGeneration(agentID string, generation uint64) {
	if s == nil {
		return
	}
	for _, id := range s.sessionsForOwner(agentID, &generation) {
		s.RemoveSession(id)
	}
}

func (s *Store) sessionsForOwner(agentID string, generation *uint64) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	seen := make(map[string]struct{})
	for _, routes := range []map[string]route{s.routes, s.observed} {
		for id, item := range routes {
			if item.agentID == agentID && (generation == nil || item.generation == *generation) {
				seen[id] = struct{}{}
			}
		}
	}
	for _, owners := range []map[string]routeOwner{s.pending, s.observedPending} {
		for id, owner := range owners {
			if owner.agentID == agentID && (generation == nil || owner.generation == *generation) {
				seen[id] = struct{}{}
			}
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	return ids
}

func (s *Store) sessionLifecycleLock(sessionID string) *sync.Mutex {
	// UUID session IDs distribute updates across fixed stripes so an ownership
	// lookup for one character does not block other routes or map snapshots.
	var hash uint64 = 1469598103934665603
	for i := 0; i < len(sessionID); i++ {
		hash ^= uint64(sessionID[i])
		hash *= 1099511628211
	}
	return &s.lifecycle[hash%uint64(len(s.lifecycle))]
}

func (s *Store) Snapshot(server string, profile mapprofile.Profile, now time.Time) []View {
	if s == nil {
		return []View{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	views := make([]View, 0, len(s.routes)+len(s.observed))
	for _, routes := range []map[string]route{s.routes, s.observed} {
		views = append(views, snapshotRoutes(routes, server, profile, now)...)
	}
	sort.Slice(views, func(i, j int) bool {
		if views[i].SessionID == views[j].SessionID {
			return views[i].CommandID < views[j].CommandID
		}
		return views[i].SessionID < views[j].SessionID
	})
	return views
}

func snapshotRoutes(routes map[string]route, server string, profile mapprofile.Profile, now time.Time) []View {
	views := make([]View, 0, len(routes))
	for sessionID, route := range routes {
		if equalFold(route.server, server) && route.datasetID != profile.DatasetID {
			delete(routes, sessionID)
			continue
		}
		if !equalFold(route.server, server) {
			continue
		}
		view := View{CommandID: route.CommandID, CharacterID: route.CharacterID, SessionID: route.SessionID,
			Sequence: route.Sequence, Server: route.server, DatasetID: route.datasetID, DatasetVersion: profile.DatasetVersion,
			Destination: route.destination, Status: route.status, Reason: route.reason, UpdatedAt: route.updatedAt, Blocks: []Block{}, Arrived: route.arrived}
		destinationZ := route.destination.Z
		if area, floor, ok := classify(profile, route.destination.Region, &destinationZ); ok {
			view.DestinationAreaID, view.DestinationFloorID = area, floor
		}
		positionScope := route.lastPosition
		if positionScope == nil && route.Source != nil {
			positionScope = route.Source
		}
		if positionScope != nil {
			if area, floor, ok := classify(profile, positionScope.Region, positionScope.Z); ok {
				view.AreaID, view.FloorID = area, floor
			}
		}
		if !route.arrived {
			blocks := make([]Block, 0, 4)
			active := Block{}
			for index, step := range route.Instructions {
				if index < route.cursor {
					continue
				}
				if step.Kind == "wait" || step.Kind == "teleport" {
					if len(active.Points) > 0 {
						blocks = append(blocks, active)
						active = Block{}
					}
					// A barrier remains active until Observe advances the cursor
					// past it. Position coincidence alone cannot reveal future
					// geometry (for example, a route that loops over itself).
					break
				}
				point, area, floor, ok := scopeStep(route, step)
				if !ok {
					if len(active.Points) > 0 {
						blocks = append(blocks, active)
						active = Block{}
					}
					// Keep the route split here. Later observations may make a
					// previously ambiguous scope resolvable.
					continue
				}
				if len(active.Points) > 0 && (!sameWalkScope(active.Points[len(active.Points)-1], active.AreaID, active.FloorID, point, area, floor)) {
					blocks = append(blocks, active)
					active = Block{}
				}
				active.AreaID, active.FloorID = area, floor
				active.Points = append(active.Points, point)
			}
			if len(active.Points) > 0 {
				blocks = append(blocks, active)
			}
			view.Blocks = blocks
			lastArea, lastFloor, lastScopeOK := "", "", false
			if route.lastPosition != nil {
				lastArea, lastFloor, lastScopeOK = classify(profile, route.lastPosition.Region, route.lastPosition.Z)
			}
			if route.lastPosition != nil && lastScopeOK && route.status == "moving" && route.cursor < len(route.Instructions) &&
				len(blocks) > 0 && (lastArea == "world" || blocks[0].Points[0].Region == route.lastPosition.Region) &&
				blocks[0].AreaID == lastArea && blocks[0].FloorID == lastFloor &&
				nearWalkBlock(&route, *route.lastPosition, route.cursor) {
				view.CurrentAnchor = &Point{Region: route.lastPosition.Region, X: route.lastPosition.X, Y: route.lastPosition.Y}
				if route.lastPosition.Z != nil {
					view.CurrentAnchor.Z = *route.lastPosition.Z
				}
			}
			if len(blocks) == 0 && route.cursor < len(route.Instructions) {
				view.Reason = "route_geometry_unavailable"
			}
		}
		if view.UpdatedAt.IsZero() {
			view.UpdatedAt = now.UTC()
		}
		view.InstructionCount = len(route.Instructions)
		view.CompletedInstructions = route.cursor
		view.Progress, view.ETASeconds = navigationProgress(route, now)
		views = append(views, view)
	}
	return views
}

// SnapshotBudget bounds navigation independently and leaves room for the rest
// of the live map frame. When geometry does not fit, it emits the record's
// status and identity without cutting a polyline.
func (s *Store) SnapshotBudget(server string, profile mapprofile.Profile, now time.Time, budget int) ([]View, int) {
	views := s.Snapshot(server, profile, now)
	if budget > MaxLiveBytes {
		budget = MaxLiveBytes
	} else if budget < 0 {
		budget = 0
	}
	used, omitted := 0, 0
	result := make([]View, 0, len(views))
	for _, view := range views {
		encoded, err := json.Marshal(view)
		if err != nil {
			omitted++
			continue
		}
		if used+len(encoded) <= budget {
			used += len(encoded)
			result = append(result, view)
			continue
		}
		view.Blocks = []Block{}
		view.CurrentAnchor = nil
		view.GeometryOmitted = true
		view.Reason = "navigation_geometry_payload_budget"
		encoded, err = json.Marshal(view)
		if err != nil || used+len(encoded) > budget {
			omitted++
			continue
		}
		used += len(encoded)
		result = append(result, view)
	}
	return result, omitted
}

// advanceWalk projects a fresh observation against only the next contiguous,
// profile-scoped walk block. A sample at a crossing that matches multiple
// distant points along the remaining route is treated as ambiguous.
func advanceWalk(current *route, position Position) (bool, bool) {
	start := current.cursor
	end := start
	for end < len(current.Instructions) && current.Instructions[end].Kind == "walk" {
		end++
	}
	if start >= end {
		return false, false
	}
	type vertex struct {
		instruction int
		point       Point
		area        string
		floor       string
	}
	vertices := make([]vertex, 0, end-start+1)
	if current.anchor != nil {
		first, area, floor, ok := scopeStep(*current, current.Instructions[start])
		if ok && closeScope(current, *current.anchor, first.Region, area, floor) {
			vertices = append(vertices, vertex{instruction: start - 1,
				point: Point{Region: current.anchor.Region, X: current.anchor.X, Y: current.anchor.Y, Z: positionZ(*current.anchor, first.Z)},
				area:  area, floor: floor})
		}
	}
	for index := start; index < end; index++ {
		point, area, floor, ok := scopeStep(*current, current.Instructions[index])
		if !ok {
			break
		}
		if len(vertices) > 0 && (!sameWalkScope(vertices[len(vertices)-1].point, vertices[len(vertices)-1].area, vertices[len(vertices)-1].floor, point, area, floor)) {
			break
		}
		vertices = append(vertices, vertex{instruction: index, point: point, area: area, floor: floor})
	}
	if len(vertices) == 0 {
		return false, true
	}
	if len(vertices) == 1 {
		if closeToMapped(position, vertices[0].point, vertices[0].area, vertices[0].floor, current.datasetID, current.server) {
			current.cursor = vertices[0].instruction + 1
			current.anchor = positionAnchor(position)
			return true, false
		}
		return false, true
	}
	type match struct {
		segment int
		t       float64
		x       float64
		y       float64
		d       float64
		order   float64
	}
	matches := make([]match, 0, len(vertices))
	for i := 0; i < len(vertices)-1; i++ {
		left, right := vertices[i].point, vertices[i+1].point
		if !sameWalkScope(left, vertices[i].area, vertices[i].floor, right, vertices[i+1].area, vertices[i+1].floor) ||
			!closeScope(current, position, left.Region, vertices[i].area, vertices[i].floor) {
			continue
		}
		t, x, y, distance := project(position.X, position.Y, left.X, left.Y, right.X, right.Y)
		if distance <= ArrivalRadius {
			matches = append(matches, match{segment: i, t: t, x: x, y: y, d: distance,
				order: float64(vertices[i].instruction) + t})
		}
	}
	if len(matches) == 0 {
		return false, true
	}
	best := matches[0]
	for _, candidate := range matches[1:] {
		if candidate.d < best.d-0.01 || (math.Abs(candidate.d-best.d) <= 0.01 && candidate.order < best.order) {
			best = candidate
		}
	}
	for _, candidate := range matches {
		if candidate.segment == best.segment || math.Abs(candidate.d-best.d) > 1 || math.Abs(candidate.order-best.order) < 0.75 {
			continue
		}
		if math.Hypot(candidate.x-best.x, candidate.y-best.y) > ArrivalRadius {
			return false, true
		}
	}
	left := vertices[best.segment].point
	right := vertices[best.segment+1]
	anchorRegion := left.Region
	if vertices[best.segment].area == "world" {
		profile, err := mapprofile.ForServer(current.server, current.datasetID)
		if err != nil {
			return false, true
		}
		var ok bool
		anchorRegion, ok = outdoorRegion(profile, best.x, best.y)
		if !ok {
			return false, true
		}
	}
	current.anchor = &Position{Region: anchorRegion, X: best.x, Y: best.y, Z: floatPointer(positionZ(position, left.Z)), At: position.At}
	if best.t >= 0.98 || math.Hypot(position.X-right.point.X, position.Y-right.point.Y) <= ArrivalRadius {
		current.cursor = right.instruction + 1
		current.anchor = &Position{Region: right.point.Region, X: right.point.X, Y: right.point.Y, Z: floatPointer(right.point.Z), At: position.At}
		return true, false
	}
	if right.instruction > current.cursor {
		current.cursor = right.instruction
		return true, false
	}
	return best.t > 0.05, false
}

func crossBarrier(current *route, position Position, previous *Position) bool {
	barrier := current.cursor
	next := nextWalk(current.Instructions, barrier+1)
	if next == nil {
		return false
	}
	point, area, floor, ok := scopeStep(*current, *next)
	if !ok || !nearWalkBlock(current, position, nextWalkIndex(current.Instructions, barrier+1)) {
		return false
	}
	prior := previous
	if prior == nil && current.Source != nil {
		prior = current.Source
	}
	if prior == nil || prior.Region == position.Region && closeXY(prior.X, prior.Y, position.X, position.Y) {
		return false
	}
	if prior.Region == point.Region && !sameScopePosition(current, *prior, area, floor) {
		return false
	}
	current.cursor = nextWalkIndex(current.Instructions, barrier+1)
	current.anchor = positionAnchor(position)
	return true
}

func nearWalkBlock(current *route, position Position, from int) bool {
	// The consumed prefix leaves a validated anchor on the segment leading to
	// the next waypoint. Keep that connector while between distant waypoints,
	// but never borrow an anchor from the other side of a transition barrier.
	if current.anchor != nil && from == current.cursor && from < len(current.Instructions) && current.Instructions[from].Kind == "walk" {
		first, area, floor, ok := scopeStep(*current, current.Instructions[from])
		anchor := Point{Region: current.anchor.Region, X: current.anchor.X, Y: current.anchor.Y, Z: positionZ(*current.anchor, first.Z)}
		if ok && closeToSegment(position, anchor, first, area, floor, area, floor, current) {
			return true
		}
	}
	for index := from; index < len(current.Instructions) && current.Instructions[index].Kind == "walk"; index++ {
		point, area, floor, ok := scopeStep(*current, current.Instructions[index])
		if !ok {
			return false
		}
		if closeToMapped(position, point, area, floor, current.datasetID, current.server) {
			return true
		}
		if index+1 < len(current.Instructions) && current.Instructions[index+1].Kind == "walk" {
			next, nextArea, nextFloor, valid := scopeStep(*current, current.Instructions[index+1])
			if valid && closeToSegment(position, point, next, area, floor, nextArea, nextFloor, current) {
				return true
			}
		}
	}
	return false
}

func closeToSegment(position Position, left, right Point, area, floor, nextArea, nextFloor string, current *route) bool {
	if !sameWalkScope(left, area, floor, right, nextArea, nextFloor) || !closeScope(current, position, left.Region, area, floor) {
		return false
	}
	_, _, _, distance := project(position.X, position.Y, left.X, left.Y, right.X, right.Y)
	return distance <= ArrivalRadius && (area == "world" || sameScopePosition(current, position, area, floor))
}

func sameScopePosition(current *route, position Position, area, floor string) bool {
	if area == "world" {
		profile, err := mapprofile.ForServer(current.server, current.datasetID)
		if err != nil {
			return false
		}
		region, ok := outdoorRegion(profile, position.X, position.Y)
		return ok && region == position.Region && !knownCaveRegion(profile, position.Region)
	}
	if position.Z == nil {
		return false
	}
	profile, err := mapprofile.ForServer(current.server, current.datasetID)
	if err != nil {
		return false
	}
	gotArea, gotFloor, ok := mapprofile.ClassifyCave(profile, &position.Region, position.Z)
	return ok && gotArea == area && gotFloor == floor
}

// Outdoor region seams are coordinates in the same validated world grid.
// Cave regions remain separate until their transition has observed evidence.
func sameWalkScope(left Point, area, floor string, right Point, nextArea, nextFloor string) bool {
	return area == nextArea && floor == nextFloor && (area == "world" || left.Region == right.Region)
}

func closeScope(current *route, position Position, expectedRegion int, area, floor string) bool {
	return (area == "world" || position.Region == expectedRegion) && sameScopePosition(current, position, area, floor)
}

func positionAnchor(position Position) *Position {
	copyPosition := position
	return &copyPosition
}

func floatPointer(value float64) *float64 { return &value }

func positionZ(position Position, fallback float64) float64 {
	if position.Z != nil {
		return *position.Z
	}
	return fallback
}

func closeToPoint(position Position, point Point) bool {
	return position.Region == point.Region && closeXY(position.X, position.Y, point.X, point.Y)
}

func project(x, y, x1, y1, x2, y2 float64) (float64, float64, float64, float64) {
	dx, dy := x2-x1, y2-y1
	lengthSquared := dx*dx + dy*dy
	t := 0.0
	if lengthSquared > 0 {
		t = math.Max(0, math.Min(1, ((x-x1)*dx+(y-y1)*dy)/lengthSquared))
	}
	px, py := x1+t*dx, y1+t*dy
	return t, px, py, math.Hypot(x-px, y-py)
}

func nextWalkIndex(steps []Instruction, from int) int {
	for i := from; i < len(steps); i++ {
		if steps[i].Kind == "walk" {
			return i
		}
	}
	return len(steps)
}

func scopeStep(route route, step Instruction) (Point, string, string, bool) {
	profile, err := mapprofile.ForServer(route.server, route.datasetID)
	if err != nil {
		return Point{}, "", "", false
	}
	// A ferry walk names its own region. Cave coordinates must use that region
	// instead of the outdoor grid or the character's starting area.
	if step.Region != nil {
		return scopeExplicitRegion(profile, *step.Region, step)
	}
	// X/Y can numerically land inside the outdoor tile grid even when they
	// came from a cave. Prefer the captured or latest observed cave context;
	// generated walk instructions do not carry a region of their own.
	context := route.lastPosition
	if context == nil && route.Source != nil {
		context = route.Source
	}
	if context == nil {
		return Point{}, "", "", false
	}
	if context != nil && context.Z != nil {
		if area, floor, ok := mapprofile.ClassifyCave(profile, &context.Region, context.Z); ok {
			if !(area == "job-temple" && floor != "1F") {
				return Point{Region: context.Region, X: step.X, Y: step.Y, Z: step.Z}, area, floor, true
			}
		}
	}
	if context != nil && knownCaveRegion(profile, context.Region) {
		return Point{}, "", "", false
	}
	if region, ok := outdoorRegion(profile, step.X, step.Y); ok {
		return Point{Region: region, X: step.X, Y: step.Y, Z: step.Z}, "world", "world", true
	}
	return Point{}, "", "", false
}

func scopeExplicitRegion(profile mapprofile.Profile, region int, step Instruction) (Point, string, string, bool) {
	z := step.Z
	if area, floor, ok := mapprofile.ClassifyCave(profile, &region, &z); ok {
		if area == "job-temple" && floor != "1F" {
			return Point{}, "", "", false
		}
		return Point{Region: region, X: step.X, Y: step.Y, Z: step.Z}, area, floor, true
	}
	if knownCaveRegion(profile, region) {
		return Point{}, "", "", false
	}
	if area, floor, ok := classify(profile, region, nil); ok && area == "world" {
		return Point{Region: region, X: step.X, Y: step.Y, Z: step.Z}, area, floor, true
	}
	return Point{}, "", "", false
}

func outdoorRegion(profile mapprofile.Profile, x, y float64) (int, bool) {
	if !coordinate(x) || !coordinate(y) || profile.CoordinateTransform != "outdoor-region-grid" {
		return 0, false
	}
	tileX := int(math.Floor(x/192.0)) + 135
	tileY := int(math.Floor(y/192.0)) + 92
	if tileX < profile.TileCatalog.MinX || tileX > profile.TileCatalog.MaxX || tileY < profile.TileCatalog.MinY || tileY > profile.TileCatalog.MaxY {
		return 0, false
	}
	return tileY*256 + tileX, true
}

func knownCaveRegion(profile mapprofile.Profile, region int) bool {
	for _, area := range profile.Areas {
		if area.Kind != "cave" {
			continue
		}
		for _, floor := range area.Floors {
			for _, known := range floor.RegionIDs {
				if known == region {
					return true
				}
			}
		}
	}
	return false
}

func classify(profile mapprofile.Profile, region int, z *float64) (string, string, bool) {
	if area, floor, ok := mapprofile.ClassifyCave(profile, &region, z); ok {
		return area, floor, true
	}
	if region > 0 {
		tileX, tileY := region%256, region/256
		if tileX >= profile.TileCatalog.MinX && tileX <= profile.TileCatalog.MaxX && tileY >= profile.TileCatalog.MinY && tileY <= profile.TileCatalog.MaxY {
			return "world", "world", true
		}
	}
	return "", "", false
}
func closeToMapped(pos Position, point Point, area, floor, dataset, server string) bool {
	if (area != "world" && pos.Region != point.Region) || dataset == "" || server == "" {
		return false
	}
	profile, err := mapprofile.ForServer(server, dataset)
	if err != nil {
		return false
	}
	if area == "world" {
		region, ok := outdoorRegion(profile, pos.X, pos.Y)
		if !ok || region != pos.Region || knownCaveRegion(profile, pos.Region) {
			return false
		}
	} else {
		if pos.Z == nil {
			return false
		}
		gotArea, gotFloor, ok := mapprofile.ClassifyCave(profile, &pos.Region, pos.Z)
		if !ok || gotArea != area || gotFloor != floor {
			return false
		}
	}
	return math.Hypot(pos.X-point.X, pos.Y-point.Y) <= ArrivalRadius
}
func closeToDestination(route route, pos Position) bool {
	dest := route.destination
	if pos.Region != dest.Region || math.Hypot(pos.X-dest.X, pos.Y-dest.Y) > ArrivalRadius {
		return false
	}
	profile, err := mapprofile.ForServer(route.server, route.datasetID)
	if err != nil {
		return false
	}
	destZ := dest.Z
	if area, floor, ok := mapprofile.ClassifyCave(profile, &dest.Region, &destZ); ok {
		if pos.Z == nil {
			return false
		}
		observedArea, observedFloor, observed := mapprofile.ClassifyCave(profile, &pos.Region, pos.Z)
		return observed && area == observedArea && floor == observedFloor
	}
	return true
}
func previousWalk(steps []Instruction, before int) *Instruction {
	for i := before - 1; i >= 0; i-- {
		if steps[i].Kind == "walk" {
			return &steps[i]
		}
	}
	return nil
}
func nextWalk(steps []Instruction, from int) *Instruction {
	for i := from; i < len(steps); i++ {
		if steps[i].Kind == "walk" {
			return &steps[i]
		}
	}
	return nil
}
func previousRegion(route route, before int) int {
	for i := before - 1; i >= 0; i-- {
		if route.Instructions[i].Kind == "walk" {
			p, _, _, ok := scopeStep(route, route.Instructions[i])
			if ok {
				return p.Region
			}
		}
	}
	return 0
}
func closeXY(x, y, a, b float64) bool { return math.Hypot(x-a, y-b) <= ArrivalRadius }
func validRegion(value int) bool      { return value != 0 && value >= -32768 && value <= 65535 }
func coordinate(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && math.Abs(value) <= 10_000_000
}
func equalFold(a, b string) bool { return strings.EqualFold(a, b) }

const (
	maxRecentMoveSamples = 6
	positionStaleAfter   = 35 * time.Second
)

func appendRecentMove(route *route, position Position) {
	if route == nil {
		return
	}
	copyPosition := position
	route.recentMoves = append(route.recentMoves, copyPosition)
	if len(route.recentMoves) > maxRecentMoveSamples {
		route.recentMoves = route.recentMoves[len(route.recentMoves)-maxRecentMoveSamples:]
	}
}

func navigationProgress(route route, now time.Time) (*float64, *int) {
	total := len(route.Instructions)
	if total == 0 || route.stopped {
		return nil, nil
	}
	completed := route.cursor
	if route.arrived {
		value := 1.0
		return &value, nil
	}
	if completed <= 0 && route.status == "waiting_for_movement" {
		return nil, nil
	}
	denominator := float64(total)
	numerator := float64(completed)
	if completed >= total {
		numerator = denominator - 1
		if numerator < 0 {
			numerator = 0
		}
	}
	if numerator > denominator-1 && denominator > 1 {
		numerator = denominator - 1
	}
	progress := numerator / denominator
	if progress < 0 {
		progress = 0
	}
	if progress > 0.99 && !route.arrived {
		progress = 0.99
	}
	eta := navigationETA(route, now)
	return &progress, eta
}

func navigationETA(route route, now time.Time) *int {
	if route.status != "moving" || route.lastPosition == nil || len(route.recentMoves) < 2 {
		return nil
	}
	if now.Sub(route.lastPosition.At) > positionStaleAfter {
		return nil
	}
	if route.cursor < len(route.Instructions) {
		step := route.Instructions[route.cursor]
		if step.Kind == "wait" || step.Kind == "teleport" {
			return nil
		}
	}
	remaining := remainingWalkDistance(route)
	if remaining <= 0 {
		return nil
	}
	speed := recentWalkSpeed(route.recentMoves)
	if speed <= 0.5 {
		return nil
	}
	seconds := int(math.Round(remaining / speed))
	if seconds < 1 {
		seconds = 1
	}
	return &seconds
}

func remainingWalkDistance(route route) float64 {
	total := 0.0
	var previous *Point
	if route.lastPosition != nil {
		point, _, _, ok := scopeStep(route, Instruction{Kind: "walk", X: route.lastPosition.X, Y: route.lastPosition.Y, Z: positionZValue(*route.lastPosition)})
		if ok {
			previous = &point
		}
	}
	for index := route.cursor; index < len(route.Instructions); index++ {
		step := route.Instructions[index]
		if step.Kind != "walk" {
			break
		}
		point, _, _, ok := scopeStep(route, step)
		if !ok {
			break
		}
		if previous != nil {
			total += math.Hypot(point.X-previous.X, point.Y-previous.Y)
		}
		previous = &point
	}
	return total
}

func positionZValue(position Position) float64 {
	if position.Z != nil {
		return *position.Z
	}
	return 0
}

func recentWalkSpeed(samples []Position) float64 {
	if len(samples) < 2 {
		return 0
	}
	left := samples[len(samples)-2]
	right := samples[len(samples)-1]
	elapsed := right.At.Sub(left.At).Seconds()
	if elapsed <= 0 {
		return 0
	}
	distance := math.Hypot(right.X-left.X, right.Y-left.Y)
	return distance / elapsed
}
