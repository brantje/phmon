package tradenexus

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"regexp"
	"strings"
	"time"
)

const (
	MaxWaypoints        = 200
	MaxGoods            = 16
	MaxGoodQuantity     = 99999
	MaxTradeDetailBytes = 256
	MaxTradeDurationS   = 86400
)

var (
	errInvalidTrade = errors.New("invalid trade report")
	waypointName    = regexp.MustCompile(`^[a-z0-9_]{1,64}$`)
	tradeTowns      = map[string]string{
		"jangan":         "Jangan",
		"donwhang":       "Donwhang",
		"hotan":          "Hotan",
		"samarkand":      "Samarkand",
		"constantinople": "Constantinople",
		"alexandria":     "Alexandria",
	}
)

type Waypoint struct {
	Name string  `json:"name"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
}

type TradeGood struct {
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
}

type TradeRoute struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type tradeName struct {
	Name string `json:"name"`
}

type TradeReport struct {
	ID         string       `json:"trade_id"`
	Server     string       `json:"server"`
	Ref        string       `json:"ref"`
	Outcome    string       `json:"outcome"`
	Reason     string       `json:"reason"`
	Route      TradeRoute   `json:"route"`
	Waypoints  []Waypoint   `json:"waypoints"`
	Goods      *[]TradeGood `json:"goods,omitempty"`
	Gold       *int32       `json:"gold,omitempty"`
	DurationS  *int         `json:"duration_s,omitempty"`
	Stars      string       `json:"stars,omitempty"`
	Detail     string       `json:"detail,omitempty"`
	Thief      *tradeName   `json:"thief,omitempty"`
	Transport  string       `json:"transport,omitempty"`
	Reporter   Reporter     `json:"reporter"`
	FinishedAt time.Time    `json:"finished_at"`
	ReceivedAt time.Time    `json:"received_at"`
}

type TradePage struct {
	Reports    []TradeReport `json:"reports"`
	NextCursor string        `json:"next_cursor,omitempty"`
	Total      int           `json:"total"`
}

type tradeEnvelope struct {
	Ref        string          `json:"ref"`
	Server     string          `json:"server"`
	Outcome    string          `json:"outcome"`
	Reason     string          `json:"reason"`
	Route      *TradeRoute     `json:"route"`
	Waypoints  json.RawMessage `json:"waypoints"`
	Goods      json.RawMessage `json:"goods"`
	Gold       json.RawMessage `json:"gold"`
	DurationS  json.RawMessage `json:"duration_s"`
	Stars      json.RawMessage `json:"stars"`
	Detail     json.RawMessage `json:"detail"`
	Thief      json.RawMessage `json:"thief"`
	Transport  json.RawMessage `json:"transport"`
	Reporter   Reporter        `json:"reporter"`
	FinishedAt string          `json:"finished_at"`
}

func reportTrade(payload []byte, now time.Time) (TradeReport, parseError) {
	var frame tradeEnvelope
	if err := json.Unmarshal(payload, &frame); err != nil {
		return TradeReport{}, parseError{Code: "invalid_message", Message: "frame must be a JSON object"}
	}
	ref, ok := requiredText(frame.Ref, MaxRefBytes)
	if !ok {
		return TradeReport{}, parseError{Code: "invalid_message", Message: "ref is required", Ref: safeRef(frame.Ref)}
	}
	server, ok := validServer(frame.Server)
	if !ok {
		return TradeReport{}, parseError{Code: "invalid_message", Message: "server is required", Ref: ref}
	}
	if !validTradeReason(frame.Outcome, frame.Reason) {
		return TradeReport{}, parseError{Code: "invalid_message", Message: "outcome and reason do not match", Ref: ref}
	}
	if frame.Route == nil {
		return TradeReport{}, parseError{Code: "invalid_message", Message: "route is required", Ref: ref}
	}
	from, ok := canonicalTown(frame.Route.From)
	if !ok {
		return TradeReport{}, parseError{Code: "invalid_message", Message: "route.from is invalid", Ref: ref}
	}
	to, ok := canonicalTown(frame.Route.To)
	if !ok {
		return TradeReport{}, parseError{Code: "invalid_message", Message: "route.to is invalid", Ref: ref}
	}
	if from == to {
		return TradeReport{}, parseError{Code: "invalid_message", Message: "route.from and route.to must differ", Ref: ref}
	}
	waypoints, waypointErr := parseWaypoints(frame.Waypoints)
	if waypointErr.Code != "" {
		waypointErr.Ref = ref
		return TradeReport{}, waypointErr
	}
	goods, goodsErr := parseGoods(frame.Goods)
	if goodsErr.Code != "" {
		goodsErr.Ref = ref
		return TradeReport{}, goodsErr
	}
	if frame.Reason == "already_empty" && goods != nil && len(*goods) > 0 {
		return TradeReport{}, parseError{Code: "invalid_message", Message: "already_empty cannot include goods", Ref: ref}
	}
	gold, goldErr := parseGold(frame.Gold)
	if goldErr.Code != "" {
		goldErr.Ref = ref
		return TradeReport{}, goldErr
	}
	duration, durationErr := parseDuration(frame.DurationS)
	if durationErr.Code != "" {
		durationErr.Ref = ref
		return TradeReport{}, durationErr
	}
	stars, starsErr := parseStars(frame.Stars)
	if starsErr.Code != "" {
		starsErr.Ref = ref
		return TradeReport{}, starsErr
	}
	detail, detailErr := parseDetail(frame.Reason, frame.Detail)
	if detailErr.Code != "" {
		detailErr.Ref = ref
		return TradeReport{}, detailErr
	}
	thief, thiefErr := parseTradeThief(frame.Reason, frame.Thief)
	if thiefErr.Code != "" {
		thiefErr.Ref = ref
		return TradeReport{}, thiefErr
	}
	transport, transportErr := parseTransport(frame.Transport)
	if transportErr.Code != "" {
		transportErr.Ref = ref
		return TradeReport{}, transportErr
	}
	reporterName, ok := requiredText(frame.Reporter.Name, MaxNameBytes)
	if !ok {
		return TradeReport{}, parseError{Code: "invalid_message", Message: "reporter.name is required", Ref: ref}
	}
	reporterApp, ok := optionalText(frame.Reporter.App, MaxNameBytes)
	if !ok {
		return TradeReport{}, parseError{Code: "invalid_message", Message: "reporter.app is too long", Ref: ref}
	}
	reporterVersion, ok := optionalText(frame.Reporter.Version, MaxNameBytes)
	if !ok {
		return TradeReport{}, parseError{Code: "invalid_message", Message: "reporter.version is too long", Ref: ref}
	}
	if frame.FinishedAt == "" {
		return TradeReport{}, parseError{Code: "invalid_message", Message: "finished_at is required", Ref: ref}
	}
	finished, err := time.Parse(time.RFC3339, frame.FinishedAt)
	if err != nil {
		finished, err = time.Parse(time.RFC3339Nano, frame.FinishedAt)
	}
	if err != nil {
		return TradeReport{}, parseError{Code: "invalid_message", Message: "finished_at is invalid", Ref: ref}
	}
	received := now.UTC()
	if finished.After(received.Add(ObservedAtSkew)) || finished.Before(received.Add(-ObservedAtSkew)) {
		finished = received
	} else {
		finished = finished.UTC()
	}
	report := TradeReport{
		Server: server, Ref: ref, Outcome: frame.Outcome, Reason: frame.Reason,
		Route: TradeRoute{From: from, To: to}, Waypoints: waypoints, Goods: goods,
		Gold: gold, DurationS: duration, Stars: stars, Detail: detail, Thief: thief, Transport: transport,
		Reporter:   Reporter{Name: reporterName, App: reporterApp, Version: reporterVersion},
		FinishedAt: finished, ReceivedAt: received,
	}
	if err := report.valid(); err != nil {
		return TradeReport{}, parseError{Code: "invalid_message", Message: "trade report is invalid", Ref: ref}
	}
	return report, parseError{}
}

func (report TradeReport) valid() error {
	if _, ok := validServer(report.Server); !ok {
		return errInvalidTrade
	}
	if _, ok := requiredText(report.Ref, MaxRefBytes); !ok {
		return errInvalidTrade
	}
	if !validTradeReason(report.Outcome, report.Reason) {
		return errInvalidTrade
	}
	from, fromOK := canonicalTown(report.Route.From)
	to, toOK := canonicalTown(report.Route.To)
	if !fromOK || !toOK || from != report.Route.From || to != report.Route.To || from == to {
		return errInvalidTrade
	}
	if report.Waypoints == nil || len(report.Waypoints) > MaxWaypoints {
		return errInvalidTrade
	}
	for _, point := range report.Waypoints {
		if !waypointName.MatchString(point.Name) || !finiteCoord(point.X) || !finiteCoord(point.Y) {
			return errInvalidTrade
		}
	}
	if report.Goods != nil {
		if len(*report.Goods) > MaxGoods {
			return errInvalidTrade
		}
		for _, good := range *report.Goods {
			if _, ok := requiredText(good.Name, MaxNameBytes); !ok || good.Quantity < 1 || good.Quantity > MaxGoodQuantity {
				return errInvalidTrade
			}
		}
	}
	if report.Reason == "already_empty" && report.Goods != nil && len(*report.Goods) > 0 {
		return errInvalidTrade
	}
	if report.DurationS != nil && (*report.DurationS < 0 || *report.DurationS > MaxTradeDurationS) {
		return errInvalidTrade
	}
	if report.Stars != "" && !validStars(report.Stars) {
		return errInvalidTrade
	}
	if report.Detail != "" {
		if report.Reason != "navigation" && report.Reason != "error" {
			return errInvalidTrade
		}
		if _, ok := requiredText(report.Detail, MaxTradeDetailBytes); !ok {
			return errInvalidTrade
		}
	}
	if report.Reason == "thief" {
		if report.Thief == nil {
			return errInvalidTrade
		}
		if _, ok := requiredText(report.Thief.Name, MaxNameBytes); !ok || report.Thief.Name != strings.TrimSpace(report.Thief.Name) {
			return errInvalidTrade
		}
	} else if report.Thief != nil {
		return errInvalidTrade
	}
	if report.Transport != "" {
		if _, ok := requiredText(report.Transport, MaxNameBytes); !ok {
			return errInvalidTrade
		}
	}
	if _, ok := requiredText(report.Reporter.Name, MaxNameBytes); !ok {
		return errInvalidTrade
	}
	if _, ok := optionalText(report.Reporter.App, MaxNameBytes); !ok {
		return errInvalidTrade
	}
	if _, ok := optionalText(report.Reporter.Version, MaxNameBytes); !ok {
		return errInvalidTrade
	}
	if report.FinishedAt.IsZero() || report.ReceivedAt.IsZero() {
		return errInvalidTrade
	}
	return nil
}

func validTradeReason(outcome, reason string) bool {
	switch outcome {
	case "success":
		return reason == "sold" || reason == "already_empty"
	case "failed":
		return reason == "thief" || reason == "navigation" || reason == "error" || reason == "transport_died"
	case "cancelled":
		return reason == "cancelled"
	default:
		return false
	}
}

func canonicalTown(value string) (string, bool) {
	town, ok := tradeTowns[strings.ToLower(strings.TrimSpace(value))]
	return town, ok
}

func validStars(value string) bool {
	switch value {
	case "1", "2", "3", "4", "5", "Max":
		return true
	default:
		return false
	}
}

func parseWaypoints(raw json.RawMessage) ([]Waypoint, parseError) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, parseError{Code: "invalid_message", Message: "waypoints is required"}
	}
	var points []json.RawMessage
	if err := json.Unmarshal(raw, &points); err != nil {
		return nil, parseError{Code: "invalid_message", Message: "waypoints is invalid"}
	}
	if len(points) > MaxWaypoints {
		return nil, parseError{Code: "invalid_message", Message: "waypoints exceeds 200 entries"}
	}
	waypoints := make([]Waypoint, 0, len(points))
	for _, point := range points {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(point, &fields); err != nil {
			return nil, parseError{Code: "invalid_message", Message: "waypoint is invalid"}
		}
		nameRaw, nameOK := fields["name"]
		xRaw, xOK := fields["x"]
		yRaw, yOK := fields["y"]
		if !nameOK || !xOK || !yOK {
			return nil, parseError{Code: "invalid_message", Message: "waypoint is invalid"}
		}
		var name string
		if err := json.Unmarshal(nameRaw, &name); err != nil || !waypointName.MatchString(name) {
			return nil, parseError{Code: "invalid_message", Message: "waypoint name is invalid"}
		}
		x, ok := parseCoord(xRaw)
		y, yValid := parseCoord(yRaw)
		if !ok || !yValid {
			return nil, parseError{Code: "invalid_message", Message: "waypoint coordinates are invalid"}
		}
		waypoints = append(waypoints, Waypoint{Name: name, X: x, Y: y})
	}
	return waypoints, parseError{}
}

func parseCoord(raw json.RawMessage) (float64, bool) {
	value, ok := parseJSONNumber(raw)
	if !ok {
		return 0, false
	}
	coord, err := value.Float64()
	if err != nil || !finiteCoord(coord) {
		return 0, false
	}
	return coord, true
}

func parseGoods(raw json.RawMessage) (*[]TradeGood, parseError) {
	if len(raw) == 0 {
		return nil, parseError{}
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, parseError{Code: "invalid_message", Message: "goods is invalid"}
	}
	var items []struct {
		Name     string       `json:"name"`
		Quantity *json.Number `json:"quantity"`
	}
	if err := unmarshalUseNumber(raw, &items); err != nil {
		return nil, parseError{Code: "invalid_message", Message: "goods is invalid"}
	}
	if len(items) > MaxGoods {
		return nil, parseError{Code: "invalid_message", Message: "goods exceeds 16 entries"}
	}
	goods := make([]TradeGood, 0, len(items))
	for _, item := range items {
		name, ok := requiredText(item.Name, MaxNameBytes)
		if !ok || item.Quantity == nil {
			return nil, parseError{Code: "invalid_message", Message: "goods entry is invalid"}
		}
		quantity, err := item.Quantity.Int64()
		if err != nil || quantity < 1 || quantity > MaxGoodQuantity {
			return nil, parseError{Code: "invalid_message", Message: "goods quantity is invalid"}
		}
		goods = append(goods, TradeGood{Name: name, Quantity: int(quantity)})
	}
	return &goods, parseError{}
}

func parseGold(raw json.RawMessage) (*int32, parseError) {
	if len(raw) == 0 {
		return nil, parseError{}
	}
	number, ok := parseJSONNumber(raw)
	if !ok {
		return nil, parseError{Code: "invalid_message", Message: "gold is invalid"}
	}
	value, err := number.Int64()
	if err != nil || value < math.MinInt32 || value > math.MaxInt32 {
		return nil, parseError{Code: "invalid_message", Message: "gold is invalid"}
	}
	gold := int32(value)
	return &gold, parseError{}
}

func parseDuration(raw json.RawMessage) (*int, parseError) {
	if len(raw) == 0 {
		return nil, parseError{}
	}
	number, ok := parseJSONNumber(raw)
	if !ok {
		return nil, parseError{Code: "invalid_message", Message: "duration_s is invalid"}
	}
	value, err := number.Int64()
	if err != nil || value < 0 || value > MaxTradeDurationS {
		return nil, parseError{Code: "invalid_message", Message: "duration_s is invalid"}
	}
	duration := int(value)
	return &duration, parseError{}
}

func parseStars(raw json.RawMessage) (string, parseError) {
	if len(raw) == 0 {
		return "", parseError{}
	}
	var stars string
	if err := json.Unmarshal(raw, &stars); err != nil || !validStars(stars) {
		return "", parseError{Code: "invalid_message", Message: "stars is invalid"}
	}
	return stars, parseError{}
}

func parseDetail(reason string, raw json.RawMessage) (string, parseError) {
	if len(raw) == 0 {
		return "", parseError{}
	}
	if reason != "navigation" && reason != "error" {
		return "", parseError{Code: "invalid_message", Message: "detail is only allowed for navigation and error"}
	}
	var detail string
	if err := json.Unmarshal(raw, &detail); err != nil {
		return "", parseError{Code: "invalid_message", Message: "detail is invalid"}
	}
	detail, ok := optionalText(detail, MaxTradeDetailBytes)
	if !ok {
		return "", parseError{Code: "invalid_message", Message: "detail is invalid"}
	}
	return detail, parseError{}
}

func parseTradeThief(reason string, raw json.RawMessage) (*tradeName, parseError) {
	if reason == "thief" {
		if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return nil, parseError{Code: "invalid_message", Message: "thief.name is required"}
		}
		var thief tradeName
		if err := json.Unmarshal(raw, &thief); err != nil {
			return nil, parseError{Code: "invalid_message", Message: "thief.name is required"}
		}
		name, ok := requiredText(thief.Name, MaxNameBytes)
		if !ok {
			return nil, parseError{Code: "invalid_message", Message: "thief.name is required"}
		}
		return &tradeName{Name: name}, parseError{}
	}
	if len(raw) > 0 {
		return nil, parseError{Code: "invalid_message", Message: "thief is only allowed for reason thief"}
	}
	return nil, parseError{}
}

func parseTransport(raw json.RawMessage) (string, parseError) {
	if len(raw) == 0 {
		return "", parseError{}
	}
	var transport string
	if err := json.Unmarshal(raw, &transport); err != nil {
		return "", parseError{Code: "invalid_message", Message: "transport is invalid"}
	}
	transport, ok := optionalText(transport, MaxNameBytes)
	if !ok {
		return "", parseError{Code: "invalid_message", Message: "transport is invalid"}
	}
	return transport, parseError{}
}

func parseJSONNumber(raw json.RawMessage) (json.Number, bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] == '"' || bytes.Equal(raw, []byte("null")) {
		return "", false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var number json.Number
	if err := decoder.Decode(&number); err != nil {
		return "", false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return "", false
	}
	return number, true
}

func unmarshalUseNumber(raw json.RawMessage, dest any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	return decoder.Decode(dest)
}

func tradeAckFrame(ref, tradeID string) []byte {
	frame := map[string]any{"v": ProtocolVersion, "type": "ack", "trade_id": tradeID}
	if ref != "" {
		frame["ref"] = ref
	}
	payload, _ := json.Marshal(frame)
	return payload
}
