package http

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	adminapi "github.com/korwlik23/go-api-starter/internal/generated/admin"
	auditapp "github.com/korwlik23/go-api-starter/internal/modules/audit/application"
	auditdomain "github.com/korwlik23/go-api-starter/internal/modules/audit/domain"
	auditports "github.com/korwlik23/go-api-starter/internal/modules/audit/ports"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
	"github.com/korwlik23/go-api-starter/internal/platform/response"
)

var ErrInvalidAuditEventHandler = errors.New("invalid audit event handler")
var auditActionPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[.][a-z][a-z0-9]*)+$`)

type EventHandler struct{ query *auditapp.ListSystemEvents }

func NewEventHandler(query *auditapp.ListSystemEvents) (*EventHandler, error) {
	if query == nil {
		return nil, ErrInvalidAuditEventHandler
	}
	return &EventHandler{query: query}, nil
}

func (handler *EventHandler) List(ctx *gin.Context) {
	session, ok := platformmiddleware.SessionFromContext(ctx.Request.Context())
	if !ok || session.UserID == nil || session.State != identitydomain.SessionStateAuthenticated {
		writeAuditProblem(ctx, http.StatusUnauthorized, "authentication_required", "Authentication is required.", nil)
		return
	}
	filter, err := parseAuditFilter(ctx)
	if err != nil {
		writeAuditProblem(ctx, http.StatusBadRequest, "invalid_request", "The audit query is invalid.", err)
		return
	}
	page, err := handler.query.Execute(ctx.Request.Context(), auditapp.ListSystemEventsCommand{ActorUserID: *session.UserID, Action: filter.Action, TargetActor: filter.ActorID, Cursor: filter.Cursor, Limit: filter.Limit})
	if err != nil {
		writeAuditError(ctx, err)
		return
	}
	items := make([]adminapi.AuditEvent, 0, len(page.Items))
	for _, event := range page.Items {
		mapped, mapErr := mapAuditEvent(event)
		if mapErr != nil {
			writeAuditProblem(ctx, http.StatusInternalServerError, "internal_error", "The audit events could not be read.", mapErr)
			return
		}
		items = append(items, mapped)
	}
	var next *string
	if page.NextCursor != nil {
		encoded, encodeErr := encodeCursor(*page.NextCursor)
		if encodeErr != nil {
			writeAuditProblem(ctx, http.StatusInternalServerError, "internal_error", "The audit events could not be read.", encodeErr)
			return
		}
		next = &encoded
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, adminapi.AuditEventListResponse{Items: items, NextCursor: next})
}

type auditFilter struct {
	Cursor  *auditports.Cursor
	Limit   int
	Action  string
	ActorID string
}

func parseAuditFilter(ctx *gin.Context) (auditFilter, error) {
	allowed := map[string]bool{"cursor": true, "limit": true, "action": true, "actor_id": true}
	for key, values := range ctx.Request.URL.Query() {
		if !allowed[key] || len(values) != 1 || strings.TrimSpace(values[0]) == "" {
			return auditFilter{}, errors.New("invalid query parameter: " + key)
		}
	}
	limit := 50
	if value, present := ctx.GetQuery("limit"); present {
		parsed, parseErr := parsePositive(value)
		if parseErr != nil || parsed > 200 {
			return auditFilter{}, errors.New("limit must be between 1 and 200")
		}
		limit = parsed
	}
	action := ctx.Query("action")
	if action != "" && (len(action) > 128 || !auditActionPattern.MatchString(action)) {
		return auditFilter{}, errors.New("invalid action")
	}
	actorID := ctx.Query("actor_id")
	if len(actorID) > 64 {
		return auditFilter{}, errors.New("invalid actor_id")
	}
	var cursor *auditports.Cursor
	if raw, present := ctx.GetQuery("cursor"); present {
		decoded, decodeErr := decodeCursor(raw)
		if decodeErr != nil {
			return auditFilter{}, decodeErr
		}
		cursor = &decoded
	}
	return auditFilter{Cursor: cursor, Limit: limit, Action: action, ActorID: actorID}, nil
}
func parsePositive(value string) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return 0, errors.New("invalid integer")
	}
	return parsed, nil
}

type cursorPayload struct {
	RecordedAt time.Time `json:"recorded_at"`
	ID         string    `json:"id"`
}

func encodeCursor(cursor auditports.Cursor) (string, error) {
	if cursor.ID == "" || cursor.RecordedAt.IsZero() {
		return "", errors.New("invalid audit cursor")
	}
	raw, err := json.Marshal(cursorPayload{RecordedAt: cursor.RecordedAt.UTC(), ID: cursor.ID})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
func decodeCursor(value string) (auditports.Cursor, error) {
	if len(value) > 512 {
		return auditports.Cursor{}, errors.New("cursor is too long")
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return auditports.Cursor{}, errors.New("invalid cursor")
	}
	var payload cursorPayload
	if err := json.Unmarshal(raw, &payload); err != nil || payload.ID == "" || payload.RecordedAt.IsZero() {
		return auditports.Cursor{}, errors.New("invalid cursor")
	}
	return auditports.Cursor{RecordedAt: payload.RecordedAt, ID: payload.ID}, nil
}

func mapAuditEvent(event auditdomain.Event) (adminapi.AuditEvent, error) {
	if event.ID == "" || event.OperationID == "" || event.OccurredAt.IsZero() || event.RecordedAt.IsZero() || event.Scope != auditdomain.ScopeSystem || !validActor(event.Actor) || event.Action == "" {
		return adminapi.AuditEvent{}, errors.New("invalid audit projection")
	}
	data := map[string]any{}
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return adminapi.AuditEvent{}, err
	}
	data = redactAuditData(data)
	body := adminapi.AuditEvent{Id: event.ID, Scope: adminapi.AuditEventScopeSystem, ActorType: adminapi.AuditEventActorType(event.Actor.Type), Action: event.Action, OperationId: event.OperationID, OccurredAt: event.OccurredAt, RecordedAt: event.RecordedAt, Data: data}
	if event.Actor.ID != "" {
		value := event.Actor.ID
		body.ActorId = &value
	}
	if event.RequestID != "" {
		value := event.RequestID
		body.RequestId = &value
	}
	return body, nil
}
func validActor(actor auditdomain.Actor) bool {
	return (actor.Type == auditdomain.ActorUser && actor.ID != "") ||
		((actor.Type == auditdomain.ActorSystem || actor.Type == auditdomain.ActorAnonymous) && actor.ID == "")
}
func redactAuditData(input map[string]any) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		normalized := strings.ToLower(key)
		if strings.Contains(normalized, "secret") || strings.Contains(normalized, "token") || strings.Contains(normalized, "password") || strings.Contains(normalized, "email") || strings.Contains(normalized, "phone") || strings.Contains(normalized, "cookie") || strings.Contains(normalized, "authorization") {
			continue
		}
		switch typed := value.(type) {
		case map[string]any:
			output[key] = redactAuditData(typed)
		case []any:
			redacted := make([]any, len(typed))
			for index, item := range typed {
				if nested, ok := item.(map[string]any); ok {
					redacted[index] = redactAuditData(nested)
				} else {
					redacted[index] = item
				}
			}
			output[key] = redacted
		default:
			output[key] = value
		}
	}
	return output
}
func writeAuditError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, auditapp.ErrAuditQueryDenied):
		writeAuditProblem(ctx, http.StatusForbidden, "permission_denied", "Permission is required.", nil)
	case errors.Is(err, auditapp.ErrInvalidAuditQuery):
		writeAuditProblem(ctx, http.StatusBadRequest, "invalid_request", "The audit query is invalid.", nil)
	default:
		writeAuditProblem(ctx, http.StatusInternalServerError, "internal_error", "The audit events could not be read.", err)
	}
}
func writeAuditProblem(ctx *gin.Context, status int, code, message string, cause error) {
	response.WriteProblem(ctx, status, platformmiddleware.RequestIDFromContext(ctx.Request.Context()), response.NewProblem(code, message, nil, cause))
}
