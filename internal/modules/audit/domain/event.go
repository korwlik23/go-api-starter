package domain

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"
)

type Scope string

const (
	ScopeSystem  Scope = "system"
	ScopeAccount Scope = "account"
)

type ActorType string

const (
	ActorUser      ActorType = "user"
	ActorSystem    ActorType = "system"
	ActorAnonymous ActorType = "anonymous"
)

var ErrInvalidEvent = errors.New("audit: invalid event")

var (
	actionPattern     = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[.][a-z][a-z0-9]*)+$`)
	requestIDPattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
	payloadKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
)

type Actor struct {
	Type ActorType
	ID   string
}

type EventInput struct {
	ID          string
	Scope       Scope
	AccountID   string
	Actor       Actor
	Action      string
	OperationID string
	RequestID   string
	Data        map[string]any
	OccurredAt  time.Time
}

type Event struct {
	ID          string
	Scope       Scope
	AccountID   string
	Actor       Actor
	Action      string
	OperationID string
	RequestID   string
	Data        json.RawMessage
	OccurredAt  time.Time
	RecordedAt  time.Time
}

func NewEvent(input EventInput) (Event, error) {
	if strings.TrimSpace(input.ID) == "" ||
		strings.TrimSpace(input.OperationID) == "" ||
		!actionPattern.MatchString(input.Action) ||
		input.OccurredAt.IsZero() ||
		!validScope(input.Scope, input.AccountID) ||
		!validActor(input.Actor) ||
		(input.RequestID != "" && !requestIDPattern.MatchString(input.RequestID)) ||
		input.Data == nil {
		return Event{}, ErrInvalidEvent
	}
	budget := payloadBudget{}
	if !validPayloadMap(input.Data, 0, &budget) {
		return Event{}, ErrInvalidEvent
	}
	encoded, err := json.Marshal(input.Data)
	if err != nil || len(encoded) < 2 || len(encoded) > 16_384 {
		return Event{}, ErrInvalidEvent
	}
	return Event{
		ID:          input.ID,
		Scope:       input.Scope,
		AccountID:   input.AccountID,
		Actor:       input.Actor,
		Action:      input.Action,
		OperationID: input.OperationID,
		RequestID:   input.RequestID,
		Data:        append(json.RawMessage(nil), encoded...),
		OccurredAt:  input.OccurredAt.UTC(),
	}, nil
}

func validScope(scope Scope, accountID string) bool {
	return (scope == ScopeSystem && accountID == "") ||
		(scope == ScopeAccount && accountID != "")
}

func validActor(actor Actor) bool {
	return (actor.Type == ActorUser && actor.ID != "") ||
		((actor.Type == ActorSystem || actor.Type == ActorAnonymous) && actor.ID == "")
}

type payloadBudget struct {
	nodes int
	keys  int
}

func validPayloadMap(
	value map[string]any,
	depth int,
	budget *payloadBudget,
) bool {
	if depth > 4 || len(value) > 64 {
		return false
	}
	for key, item := range value {
		budget.keys++
		if budget.keys > 64 ||
			!payloadKeyPattern.MatchString(key) ||
			sensitivePayloadKey(key) ||
			!validPayloadValue(item, depth+1, budget) {
			return false
		}
	}
	return true
}

func validPayloadValue(value any, depth int, budget *payloadBudget) bool {
	budget.nodes++
	if budget.nodes > 128 || depth > 4 {
		return false
	}
	switch typed := value.(type) {
	case nil, bool,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64, json.Number:
		return true
	case string:
		return len(typed) <= 512
	case map[string]any:
		return validPayloadMap(typed, depth, budget)
	case []any:
		if len(typed) > 32 {
			return false
		}
		for _, item := range typed {
			if !validPayloadValue(item, depth+1, budget) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func sensitivePayloadKey(key string) bool {
	normalized := strings.ToLower(key)
	for _, forbidden := range []string{
		"password",
		"passwd",
		"secret",
		"token",
		"cookie",
		"authorization",
		"email",
		"phone",
		"address",
		"first_name",
		"last_name",
		"full_name",
		"client_ip",
		"ip_address",
	} {
		if strings.Contains(normalized, forbidden) {
			return true
		}
	}
	return false
}
