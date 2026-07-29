package http

import (
	"errors"
	nethttp "net/http"
	"time"

	"github.com/gin-gonic/gin"
	adminapi "github.com/korwlik23/go-api-starter/internal/generated/admin"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

var ErrInvalidSessionQueryHandler = errors.New(
	"invalid session query handler dependencies",
)

type SessionQueryHandler struct {
	users    ports.UserByIDReader
	sessions ports.SessionManagementRepository
	now      func() time.Time
}

func NewSessionQueryHandler(
	users ports.UserByIDReader,
	sessions ports.SessionManagementRepository,
	now func() time.Time,
) (*SessionQueryHandler, error) {
	if users == nil || sessions == nil || now == nil {
		return nil, ErrInvalidSessionQueryHandler
	}
	return &SessionQueryHandler{users: users, sessions: sessions, now: now}, nil
}

func (handler *SessionQueryHandler) Current(ctx *gin.Context) {
	session, ok := authenticatedSession(ctx)
	if !ok {
		return
	}
	user, err := handler.users.FindByID(ctx.Request.Context(), *session.UserID)
	if err != nil {
		writeLoginProblem(
			ctx,
			nethttp.StatusInternalServerError,
			"internal_error",
			"The request could not be completed.",
			err,
		)
		return
	}
	body := adminapi.CurrentSessionResponse{
		SessionId: session.ID,
		State: adminapi.CurrentSessionResponseState(
			identitydomain.SessionStateAuthenticated,
		),
	}
	body.Principal.Id = user.ID
	body.Principal.Email = openapi_types.Email(user.Email)
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(nethttp.StatusOK, body)
}

func (handler *SessionQueryHandler) List(ctx *gin.Context) {
	current, ok := authenticatedSession(ctx)
	if !ok {
		return
	}
	sessions, err := handler.sessions.ListActiveByUser(
		ctx.Request.Context(),
		*current.UserID,
		handler.now(),
	)
	if err != nil {
		writeLoginProblem(
			ctx,
			nethttp.StatusInternalServerError,
			"internal_error",
			"The request could not be completed.",
			err,
		)
		return
	}
	body := adminapi.SessionListResponse{
		Items: make([]struct {
			CreatedAt  time.Time `json:"created_at"`
			Current    bool      `json:"current"`
			Id         string    `json:"id"`
			LastSeenAt time.Time `json:"last_seen_at"`
		}, 0, len(sessions)),
	}
	for _, session := range sessions {
		body.Items = append(body.Items, struct {
			CreatedAt  time.Time `json:"created_at"`
			Current    bool      `json:"current"`
			Id         string    `json:"id"`
			LastSeenAt time.Time `json:"last_seen_at"`
		}{
			CreatedAt:  session.CreatedAt,
			Current:    session.ID == current.ID,
			Id:         session.ID,
			LastSeenAt: session.CreatedAt,
		})
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(nethttp.StatusOK, body)
}

func (handler *SessionQueryHandler) Revoke(ctx *gin.Context) {
	current, ok := authenticatedSession(ctx)
	if !ok {
		return
	}
	targetID := ctx.Param("session_id")
	if targetID == "" || targetID == current.ID {
		writeLoginProblem(
			ctx,
			nethttp.StatusConflict,
			"session_revocation_conflict",
			"The target session cannot be revoked.",
			nil,
		)
		return
	}
	revoked, err := handler.sessions.RevokeOwnedByID(
		ctx.Request.Context(),
		*current.UserID,
		targetID,
		handler.now(),
	)
	if err != nil {
		writeLoginProblem(
			ctx,
			nethttp.StatusInternalServerError,
			"internal_error",
			"The request could not be completed.",
			err,
		)
		return
	}
	if !revoked {
		writeLoginProblem(
			ctx,
			nethttp.StatusConflict,
			"session_revocation_conflict",
			"The target session cannot be revoked.",
			nil,
		)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(nethttp.StatusOK, adminapi.SessionRevocationResult{
		Revoked:         adminapi.SessionRevocationResultRevoked(true),
		TargetSessionId: targetID,
	})
}

func authenticatedSession(ctx *gin.Context) (identitydomain.Session, bool) {
	session, ok := platformmiddleware.SessionFromContext(ctx.Request.Context())
	if !ok {
		writeLoginProblem(
			ctx,
			nethttp.StatusUnauthorized,
			"authentication_required",
			"Authentication is required.",
			nil,
		)
		return identitydomain.Session{}, false
	}
	if session.State != identitydomain.SessionStateAuthenticated ||
		session.UserID == nil {
		writeLoginProblem(
			ctx,
			nethttp.StatusUnauthorized,
			"authentication_required",
			"Authentication is required.",
			nil,
		)
		return identitydomain.Session{}, false
	}
	return session, true
}
