package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	accountports "github.com/korwlik23/go-api-starter/internal/modules/accounts/ports"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	menusapp "github.com/korwlik23/go-api-starter/internal/modules/navigation/application/menus"
	navigationdomain "github.com/korwlik23/go-api-starter/internal/modules/navigation/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/navigation/ports"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
	"github.com/korwlik23/go-api-starter/internal/platform/response"
)

var ErrInvalidMenuHandler = errors.New("invalid navigation menu handler dependencies")

type menuManager interface {
	List(context.Context, menusapp.ListInput) (ports.MenuListPage, error)
	Get(context.Context, string, string) (navigationdomain.Menu, error)
	Create(context.Context, menusapp.CreateInput) (navigationdomain.Menu, error)
	Update(context.Context, menusapp.UpdateInput) (navigationdomain.Menu, error)
	Delete(context.Context, menusapp.DeleteInput) error
	Reorder(context.Context, menusapp.ReorderInput) (navigationdomain.Menu, error)
}

type menuAccountResolver interface {
	Execute(context.Context, string) (accountports.ResolvedAccount, error)
}

type MenuHandler struct {
	manager  menuManager
	accounts menuAccountResolver
}

func NewMenuHandler(manager menuManager, accounts menuAccountResolver) (*MenuHandler, error) {
	if manager == nil || accounts == nil {
		return nil, ErrInvalidMenuHandler
	}
	return &MenuHandler{manager: manager, accounts: accounts}, nil
}

type menuItemRequest struct {
	ID         string                      `json:"id,omitempty"`
	ParentID   string                      `json:"parent_id,omitempty"`
	Label      string                      `json:"label"`
	TargetKind navigationdomain.TargetKind `json:"target_kind"`
	Target     string                      `json:"target"`
	Position   int64                       `json:"position"`
	Enabled    bool                        `json:"enabled"`
	Children   []menuItemRequest           `json:"children,omitempty"`
}

type menuMutationRequest struct {
	LocaleID string            `json:"locale_id"`
	Key      string            `json:"key"`
	Name     string            `json:"name"`
	Enabled  bool              `json:"enabled"`
	Items    []menuItemRequest `json:"items"`
}

type menuUpdateRequest struct {
	menuMutationRequest
	ExpectedVersion int64 `json:"expected_version"`
}

type menuDeleteRequest struct {
	ExpectedVersion int64 `json:"expected_version"`
}

type menuReorderRequest struct {
	ExpectedVersion int64                `json:"expected_version"`
	Items           []reorderItemRequest `json:"items"`
}

type reorderItemRequest struct {
	ID       string `json:"id"`
	ParentID string `json:"parent_id,omitempty"`
	Position int64  `json:"position"`
}

type menuResponse struct {
	ID        string             `json:"id"`
	AccountID string             `json:"account_id"`
	LocaleID  string             `json:"locale_id"`
	Key       string             `json:"key"`
	Name      string             `json:"name"`
	Enabled   bool               `json:"enabled"`
	Version   int64              `json:"version"`
	Items     []menuItemResponse `json:"items"`
	CreatedAt string             `json:"created_at"`
	UpdatedAt string             `json:"updated_at"`
}

type menuItemResponse struct {
	ID         string                      `json:"id"`
	ParentID   string                      `json:"parent_id,omitempty"`
	Label      string                      `json:"label"`
	TargetKind navigationdomain.TargetKind `json:"target_kind"`
	Target     string                      `json:"target"`
	Position   int64                       `json:"position"`
	Enabled    bool                        `json:"enabled"`
	Version    int64                       `json:"version"`
	Children   []menuItemResponse          `json:"children,omitempty"`
}

type menuListResponse struct {
	Items []menuResponse      `json:"items"`
	Next  *menuCursorResponse `json:"next,omitempty"`
}

type menuCursorResponse struct {
	UpdatedAt string `json:"updated_at"`
	ID        string `json:"id"`
}

func (handler *MenuHandler) List(ctx *gin.Context) {
	_, accountID, ok := handler.principal(ctx)
	if !ok {
		return
	}
	limit := 25
	if raw := strings.TrimSpace(ctx.Query("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 {
			writeMenuProblem(ctx, http.StatusBadRequest, "invalid_request", "The menu list request is invalid.", nil)
			return
		}
		limit = parsed
	}
	var cursor *ports.MenuCursor
	if rawUpdated, rawID := strings.TrimSpace(ctx.Query("after_updated_at")), strings.TrimSpace(ctx.Query("after_id")); rawUpdated != "" || rawID != "" {
		updatedAt, err := time.Parse(time.RFC3339Nano, rawUpdated)
		if err != nil || rawID == "" {
			writeMenuProblem(ctx, http.StatusBadRequest, "invalid_request", "The menu cursor is invalid.", nil)
			return
		}
		cursor = &ports.MenuCursor{UpdatedAt: updatedAt, ID: rawID}
	}
	var enabled *bool
	if raw := strings.TrimSpace(ctx.Query("enabled")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			writeMenuProblem(ctx, http.StatusBadRequest, "invalid_request", "The menu visibility filter is invalid.", nil)
			return
		}
		enabled = &parsed
	}
	page, err := handler.manager.List(ctx.Request.Context(), menusapp.ListInput{AccountID: accountID, LocaleID: strings.TrimSpace(ctx.Query("locale_id")), Enabled: enabled, After: cursor, Limit: limit})
	if err != nil {
		writeMenuError(ctx, err)
		return
	}
	items := make([]menuResponse, 0, len(page.Items))
	for _, menu := range page.Items {
		items = append(items, toMenuResponse(menu))
	}
	result := menuListResponse{Items: items}
	if page.Next != nil {
		result.Next = &menuCursorResponse{UpdatedAt: page.Next.UpdatedAt.UTC().Format(time.RFC3339Nano), ID: page.Next.ID}
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, result)
}

func (handler *MenuHandler) Get(ctx *gin.Context) {
	_, accountID, ok := handler.principal(ctx)
	if !ok {
		return
	}
	menu, err := handler.manager.Get(ctx.Request.Context(), accountID, strings.TrimSpace(ctx.Param("menu_id")))
	if err != nil {
		writeMenuError(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, toMenuResponse(menu))
}

func (handler *MenuHandler) Create(ctx *gin.Context) {
	userID, accountID, ok := handler.principal(ctx)
	if !ok || !handler.prepareJSON(ctx) {
		return
	}
	var request menuMutationRequest
	if !handler.decodeJSON(ctx, &request) {
		return
	}
	menu, err := handler.manager.Create(ctx.Request.Context(), menusapp.CreateInput{AccountID: accountID, ActorUserID: userID, RequestID: platformmiddleware.RequestIDFromContext(ctx.Request.Context()), LocaleID: request.LocaleID, Key: request.Key, Name: request.Name, Enabled: request.Enabled, Items: toItemInputs(request.Items)})
	if err != nil {
		writeMenuError(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusCreated, toMenuResponse(menu))
}

func (handler *MenuHandler) Update(ctx *gin.Context) {
	userID, accountID, ok := handler.principal(ctx)
	if !ok || !handler.prepareJSON(ctx) {
		return
	}
	var request menuUpdateRequest
	if !handler.decodeJSON(ctx, &request) {
		return
	}
	menu, err := handler.manager.Update(ctx.Request.Context(), menusapp.UpdateInput{AccountID: accountID, ActorUserID: userID, RequestID: platformmiddleware.RequestIDFromContext(ctx.Request.Context()), MenuID: strings.TrimSpace(ctx.Param("menu_id")), ExpectedVersion: request.ExpectedVersion, LocaleID: request.LocaleID, Key: request.Key, Name: request.Name, Enabled: request.Enabled, Items: toItemInputs(request.Items)})
	if err != nil {
		writeMenuError(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, toMenuResponse(menu))
}

func (handler *MenuHandler) Delete(ctx *gin.Context) {
	userID, accountID, ok := handler.principal(ctx)
	if !ok || !handler.prepareJSON(ctx) {
		return
	}
	var request menuDeleteRequest
	if !handler.decodeJSON(ctx, &request) {
		return
	}
	err := handler.manager.Delete(ctx.Request.Context(), menusapp.DeleteInput{AccountID: accountID, ActorUserID: userID, RequestID: platformmiddleware.RequestIDFromContext(ctx.Request.Context()), MenuID: strings.TrimSpace(ctx.Param("menu_id")), ExpectedVersion: request.ExpectedVersion})
	if err != nil {
		writeMenuError(ctx, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func (handler *MenuHandler) Reorder(ctx *gin.Context) {
	userID, accountID, ok := handler.principal(ctx)
	if !ok || !handler.prepareJSON(ctx) {
		return
	}
	var request menuReorderRequest
	if !handler.decodeJSON(ctx, &request) {
		return
	}
	items := make([]menusapp.ReorderItem, 0, len(request.Items))
	for _, item := range request.Items {
		items = append(items, menusapp.ReorderItem{ID: item.ID, ParentID: item.ParentID, Position: item.Position})
	}
	menu, err := handler.manager.Reorder(ctx.Request.Context(), menusapp.ReorderInput{AccountID: accountID, ActorUserID: userID, RequestID: platformmiddleware.RequestIDFromContext(ctx.Request.Context()), MenuID: strings.TrimSpace(ctx.Param("menu_id")), ExpectedVersion: request.ExpectedVersion, Items: items})
	if err != nil {
		writeMenuError(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, toMenuResponse(menu))
}

func (handler *MenuHandler) principal(ctx *gin.Context) (string, string, bool) {
	session, ok := platformmiddleware.SessionFromContext(ctx.Request.Context())
	if !ok || session.State != identitydomain.SessionStateAuthenticated || session.UserID == nil || strings.TrimSpace(*session.UserID) == "" {
		writeMenuProblem(ctx, http.StatusUnauthorized, "authentication_required", "Authentication is required.", nil)
		return "", "", false
	}
	resolved, err := handler.accounts.Execute(ctx.Request.Context(), *session.UserID)
	if err != nil || strings.TrimSpace(resolved.Account.ID) == "" {
		writeMenuProblem(ctx, http.StatusForbidden, "permission_denied", "Permission is required.", nil)
		return "", "", false
	}
	return *session.UserID, resolved.Account.ID, true
}

func (handler *MenuHandler) prepareJSON(ctx *gin.Context) bool {
	if strings.TrimSpace(platformmiddleware.RequestIDFromContext(ctx.Request.Context())) == "" {
		writeMenuProblem(ctx, http.StatusBadRequest, "invalid_request", "A request ID is required for menu mutations.", nil)
		return false
	}
	mediaType, _, err := mime.ParseMediaType(ctx.GetHeader("Content-Type"))
	if err != nil || !strings.EqualFold(mediaType, "application/json") {
		writeMenuProblem(ctx, http.StatusUnsupportedMediaType, "unsupported_media_type", "Menu mutations require application/json.", nil)
		return false
	}
	return true
}

func (handler *MenuHandler) decodeJSON(ctx *gin.Context, destination any) bool {
	const maxBody = 2 << 20
	raw, err := io.ReadAll(io.LimitReader(ctx.Request.Body, maxBody+1))
	if err != nil || len(raw) > maxBody || rejectDuplicateJSONKeys(raw) != nil {
		writeMenuProblem(ctx, http.StatusBadRequest, "invalid_request", "The menu request is invalid.", nil)
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeMenuProblem(ctx, http.StatusBadRequest, "invalid_request", "The menu request is invalid.", nil)
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeMenuProblem(ctx, http.StatusBadRequest, "invalid_request", "The menu request is invalid.", nil)
		return false
	}
	return true
}

func rejectDuplicateJSONKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var walk func() error
	walk = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		if delimiter, ok := token.(json.Delim); ok {
			switch delimiter {
			case '{':
				seen := map[string]struct{}{}
				for decoder.More() {
					keyToken, keyErr := decoder.Token()
					if keyErr != nil {
						return keyErr
					}
					key, ok := keyToken.(string)
					if !ok {
						return errors.New("invalid JSON key")
					}
					if _, exists := seen[key]; exists {
						return errors.New("duplicate JSON key")
					}
					seen[key] = struct{}{}
					if err := walk(); err != nil {
						return err
					}
				}
				_, err = decoder.Token()
				return err
			case '[':
				for decoder.More() {
					if err := walk(); err != nil {
						return err
					}
				}
				_, err = decoder.Token()
				return err
			}
		}
		return nil
	}
	if err := walk(); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("multiple JSON values")
	}
	return nil
}

func writeMenuProblem(ctx *gin.Context, status int, code, message string, cause error) {
	response.WriteProblem(ctx, status, platformmiddleware.RequestIDFromContext(ctx.Request.Context()), response.NewProblem(code, message, nil, cause))
}

func writeMenuError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, ports.ErrMenuNotFound):
		writeMenuProblem(ctx, http.StatusNotFound, "not_found", "The navigation menu was not found.", nil)
	case errors.Is(err, ports.ErrMenuConflict):
		writeMenuProblem(ctx, http.StatusConflict, "conflict", "The navigation menu changed; reload before retrying.", nil)
	case errors.Is(err, menusapp.ErrInvalidMenuMutation), errors.Is(err, navigationdomain.ErrInvalidMenu), errors.Is(err, navigationdomain.ErrInvalidMenuItem), errors.Is(err, navigationdomain.ErrMenuTreeCycle), errors.Is(err, navigationdomain.ErrMenuTreeTooDeep), errors.Is(err, navigationdomain.ErrMenuTreeParent), errors.Is(err, navigationdomain.ErrMenuTreeDuplicate), errors.Is(err, navigationdomain.ErrMenuTreeOrder):
		writeMenuProblem(ctx, http.StatusUnprocessableEntity, "invalid_request", "The navigation menu request is invalid.", nil)
	case errors.Is(err, menusapp.ErrInvalidMenuQuery), errors.Is(err, ports.ErrInvalidMenuQuery):
		writeMenuProblem(ctx, http.StatusBadRequest, "invalid_request", "The navigation menu query is invalid.", nil)
	default:
		writeMenuProblem(ctx, http.StatusInternalServerError, "internal_error", "The navigation menu request could not be completed.", nil)
	}
}

func toItemInputs(items []menuItemRequest) []menusapp.ItemInput {
	result := make([]menusapp.ItemInput, 0, len(items))
	for _, item := range items {
		result = append(result, menusapp.ItemInput{ID: item.ID, ParentID: item.ParentID, Label: item.Label, TargetKind: item.TargetKind, Target: item.Target, Position: item.Position, Enabled: item.Enabled, Children: toItemInputs(item.Children)})
	}
	return result
}

func toMenuResponse(menu navigationdomain.Menu) menuResponse {
	items := make([]menuItemResponse, 0, len(menu.Items))
	for _, item := range menu.Items {
		items = append(items, toMenuItemResponse(item))
	}
	return menuResponse{ID: menu.ID, AccountID: menu.AccountID, LocaleID: menu.LocaleID, Key: menu.Key, Name: menu.Name, Enabled: menu.Enabled, Version: menu.Version, Items: items, CreatedAt: menu.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: menu.UpdatedAt.UTC().Format(time.RFC3339Nano)}
}

func toMenuItemResponse(item navigationdomain.MenuItem) menuItemResponse {
	children := make([]menuItemResponse, 0, len(item.Children))
	for _, child := range item.Children {
		children = append(children, toMenuItemResponse(child))
	}
	return menuItemResponse{ID: item.ID, ParentID: item.ParentID, Label: item.Label, TargetKind: item.TargetKind, Target: item.Target, Position: item.Position, Enabled: item.Enabled, Version: item.Version, Children: children}
}
