package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	nethttp "net/http"
	"strings"

	"github.com/gin-gonic/gin"
	accountports "github.com/korwlik23/go-api-starter/internal/modules/accounts/ports"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	contentapp "github.com/korwlik23/go-api-starter/internal/modules/publishing/application/content"
	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
	"github.com/korwlik23/go-api-starter/internal/platform/response"
)

var ErrInvalidContentHandler = errors.New("invalid content handler dependencies")

type contentManager interface {
	Create(context.Context, contentapp.CreateInput) (ports.ContentDetail, error)
	Update(context.Context, contentapp.UpdateInput) (ports.ContentDetail, error)
	Delete(context.Context, contentapp.DeleteInput) error
}

type contentReader interface {
	List(context.Context, contentapp.ListInput) (ports.ContentListPage, error)
	Get(context.Context, contentapp.GetInput) (ports.ContentDetail, error)
}

type accountResolver interface {
	Execute(context.Context, string) (accountports.ResolvedAccount, error)
}

type ContentHandler struct {
	manager  contentManager
	queries  contentReader
	accounts accountResolver
}

func NewContentHandler(
	manager contentManager,
	accounts accountResolver,
) (*ContentHandler, error) {
	return NewContentHandlerWithQueries(manager, nil, accounts)
}

func NewContentHandlerWithQueries(
	manager contentManager,
	queries contentReader,
	accounts accountResolver,
) (*ContentHandler, error) {
	if manager == nil || accounts == nil {
		return nil, ErrInvalidContentHandler
	}
	return &ContentHandler{manager: manager, queries: queries, accounts: accounts}, nil
}

type contentMetadataRequest struct {
	Title          string         `json:"title"`
	Description    string         `json:"description"`
	CanonicalURL   string         `json:"canonical_url"`
	Robots         string         `json:"robots"`
	StructuredData map[string]any `json:"structured_data"`
}

type contentGeoRequest struct {
	Region   string `json:"region"`
	Locality string `json:"locality"`
}

type contentAEORequest struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

type contentMutationRequest struct {
	Kind            string                   `json:"kind"`
	ContentKey      string                   `json:"content_key"`
	TranslationID   string                   `json:"translation_id"`
	Locale          string                   `json:"locale"`
	Slug            string                   `json:"slug"`
	Path            string                   `json:"path"`
	Title           string                   `json:"title"`
	Excerpt         string                   `json:"excerpt"`
	Blocks          []publishingdomain.Block `json:"blocks"`
	SEO             contentMetadataRequest   `json:"seo"`
	GEO             contentGeoRequest        `json:"geo"`
	AEO             contentAEORequest        `json:"aeo"`
	ExpectedVersion int64                    `json:"expected_version"`
}

type contentResponse struct {
	ContentID          string                   `json:"content_id"`
	TranslationID      string                   `json:"translation_id"`
	AccountID          string                   `json:"account_id"`
	LocaleID           string                   `json:"locale_id"`
	Kind               publishingdomain.Kind    `json:"kind"`
	ContentKey         string                   `json:"content_key"`
	ContentStatus      publishingdomain.Status  `json:"content_status"`
	ContentVersion     int64                    `json:"content_version"`
	TranslationStatus  publishingdomain.Status  `json:"translation_status"`
	TranslationVersion int64                    `json:"translation_version"`
	Slug               string                   `json:"slug"`
	Path               string                   `json:"path"`
	Title              string                   `json:"title"`
	Excerpt            string                   `json:"excerpt"`
	Blocks             []publishingdomain.Block `json:"blocks"`
	SEO                contentMetadataResponse  `json:"seo"`
	GEO                contentGeoRequest        `json:"geo"`
	AEO                contentAEORequest        `json:"aeo"`
}

type contentMetadataResponse struct {
	Title          string         `json:"title"`
	Description    string         `json:"description"`
	CanonicalURL   string         `json:"canonical_url"`
	Robots         string         `json:"robots"`
	StructuredData map[string]any `json:"structured_data,omitempty"`
}

func (handler *ContentHandler) Create(ctx *gin.Context) {
	handler.create(ctx, nil)
}

func (handler *ContentHandler) CreatePage(ctx *gin.Context) {
	kind := publishingdomain.KindPage
	handler.create(ctx, &kind)
}

func (handler *ContentHandler) CreatePost(ctx *gin.Context) {
	kind := publishingdomain.KindPost
	handler.create(ctx, &kind)
}

func (handler *ContentHandler) create(ctx *gin.Context, forcedKind *publishingdomain.Kind) {
	userID, accountID, ok := handler.principal(ctx)
	if !ok {
		return
	}
	var request contentMutationRequest
	if !handler.prepareMutation(ctx, &request) {
		return
	}
	if forcedKind != nil {
		if request.Kind != "" && publishingdomain.Kind(request.Kind) != *forcedKind {
			writeContentProblem(ctx, nethttp.StatusBadRequest, errors.New("route content kind does not match request"))
			return
		}
		request.Kind = string(*forcedKind)
	}
	result, err := handler.manager.Create(ctx.Request.Context(), contentapp.CreateInput{
		AccountID: accountID, ActorUserID: userID,
		RequestID: platformmiddleware.RequestIDFromContext(ctx.Request.Context()),
		Kind:      publishingdomain.Kind(request.Kind), ContentKey: request.ContentKey,
		Locale: request.Locale, Slug: request.Slug, Path: request.Path,
		Title: request.Title, Excerpt: request.Excerpt, Blocks: request.Blocks,
		SEO: publishingdomain.SEOFields{
			Title: request.SEO.Title, Description: request.SEO.Description,
			CanonicalURL: request.SEO.CanonicalURL, Robots: request.SEO.Robots,
			StructuredData: request.SEO.StructuredData,
		},
		GEO: publishingdomain.GEOFields{
			Region: request.GEO.Region, Locality: request.GEO.Locality,
		},
		AEO: publishingdomain.AEOFields{
			Question: request.AEO.Question, Answer: request.AEO.Answer,
		},
	})
	if err != nil {
		writeContentMutationError(ctx, err)
		return
	}
	ctx.JSON(nethttp.StatusCreated, toContentResponse(result))
}

func (handler *ContentHandler) Update(ctx *gin.Context) {
	userID, accountID, ok := handler.principal(ctx)
	if !ok {
		return
	}
	var request contentMutationRequest
	if !handler.prepareMutation(ctx, &request) {
		return
	}
	result, err := handler.manager.Update(ctx.Request.Context(), contentapp.UpdateInput{
		AccountID: accountID, ActorUserID: userID,
		RequestID: platformmiddleware.RequestIDFromContext(ctx.Request.Context()),
		ContentID: contentIDParam(ctx), TranslationID: request.TranslationID,
		Locale: request.Locale, Slug: request.Slug, Path: request.Path,
		Title: request.Title, Excerpt: request.Excerpt, Blocks: request.Blocks,
		ExpectedVersion: request.ExpectedVersion,
		SEO: publishingdomain.SEOFields{
			Title: request.SEO.Title, Description: request.SEO.Description,
			CanonicalURL: request.SEO.CanonicalURL, Robots: request.SEO.Robots,
			StructuredData: request.SEO.StructuredData,
		},
		GEO: publishingdomain.GEOFields{
			Region: request.GEO.Region, Locality: request.GEO.Locality,
		},
		AEO: publishingdomain.AEOFields{
			Question: request.AEO.Question, Answer: request.AEO.Answer,
		},
	})
	if err != nil {
		writeContentMutationError(ctx, err)
		return
	}
	ctx.JSON(nethttp.StatusOK, toContentResponse(result))
}

func (handler *ContentHandler) Delete(ctx *gin.Context) {
	userID, accountID, ok := handler.principal(ctx)
	if !ok {
		return
	}
	var request struct {
		TranslationID   string `json:"translation_id"`
		LocaleID        string `json:"locale_id"`
		ExpectedVersion int64  `json:"expected_version"`
	}
	if !handler.prepareMutation(ctx, &request) {
		return
	}
	err := handler.manager.Delete(ctx.Request.Context(), contentapp.DeleteInput{
		AccountID: accountID, ActorUserID: userID,
		RequestID: platformmiddleware.RequestIDFromContext(ctx.Request.Context()),
		ContentID: contentIDParam(ctx), TranslationID: request.TranslationID,
		LocaleID: request.LocaleID, ExpectedVersion: request.ExpectedVersion,
	})
	if err != nil {
		writeContentMutationError(ctx, err)
		return
	}
	ctx.Status(nethttp.StatusNoContent)
}

func contentIDParam(ctx *gin.Context) string {
	for _, key := range []string{"content_id", "page_id", "post_id"} {
		if value := strings.TrimSpace(ctx.Param(key)); value != "" {
			return value
		}
	}
	return ""
}

func (handler *ContentHandler) prepareMutation(ctx *gin.Context, destination any) bool {
	requestID := platformmiddleware.RequestIDFromContext(ctx.Request.Context())
	if strings.TrimSpace(requestID) == "" {
		writeContentProblemCode(
			ctx,
			nethttp.StatusBadRequest,
			"invalid_request",
			"A request ID is required for content mutations.",
			nil,
		)
		return false
	}
	if err := requireJSONContentType(ctx); err != nil {
		writeContentProblemCode(
			ctx,
			nethttp.StatusUnsupportedMediaType,
			"unsupported_media_type",
			"Content mutations require application/json.",
			err,
		)
		return false
	}
	if err := decodeStrictJSON(ctx, destination); err != nil {
		writeContentProblemCode(ctx, nethttp.StatusBadRequest, "invalid_request", "The content request is invalid.", err)
		return false
	}
	return true
}

func (handler *ContentHandler) principal(ctx *gin.Context) (string, string, bool) {
	session, ok := platformmiddleware.SessionFromContext(ctx.Request.Context())
	if !ok || session.State != identitydomain.SessionStateAuthenticated || session.UserID == nil {
		response.WriteProblem(
			ctx,
			nethttp.StatusUnauthorized,
			platformmiddleware.RequestIDFromContext(ctx.Request.Context()),
			response.NewProblem("authentication_required", "Authentication is required.", nil, nil),
		)
		return "", "", false
	}
	resolved, err := handler.accounts.Execute(ctx.Request.Context(), *session.UserID)
	if err != nil || resolved.Account.ID == "" {
		response.WriteProblem(
			ctx,
			nethttp.StatusForbidden,
			platformmiddleware.RequestIDFromContext(ctx.Request.Context()),
			response.NewProblem("permission_denied", "Permission is required.", nil, err),
		)
		return "", "", false
	}
	return *session.UserID, resolved.Account.ID, true
}

func decodeStrictJSON(ctx *gin.Context, destination any) error {
	const maxContentMutationBytes = 2 << 20
	raw, err := io.ReadAll(io.LimitReader(ctx.Request.Body, maxContentMutationBytes+1))
	if err != nil {
		return err
	}
	if len(raw) > maxContentMutationBytes {
		return errors.New("content request is too large")
	}
	if err := rejectDuplicateJSONKeys(raw); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func requireJSONContentType(ctx *gin.Context) error {
	value := strings.TrimSpace(ctx.GetHeader("Content-Type"))
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil || mediaType != "application/json" {
		return errors.New("content type must be application/json")
	}
	return nil
}

func rejectDuplicateJSONKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var walk func() error
	walk = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch delimiter := token.(type) {
		case json.Delim:
			switch delimiter {
			case '{':
				seen := make(map[string]struct{})
				for decoder.More() {
					keyToken, keyErr := decoder.Token()
					if keyErr != nil {
						return keyErr
					}
					key, ok := keyToken.(string)
					if !ok {
						return errors.New("object key must be a string")
					}
					if _, exists := seen[key]; exists {
						return errors.New("duplicate JSON object key: " + key)
					}
					seen[key] = struct{}{}
					if err := walk(); err != nil {
						return err
					}
				}
				end, endErr := decoder.Token()
				if endErr != nil || end != json.Delim('}') {
					return errors.New("invalid JSON object")
				}
			case '[':
				for decoder.More() {
					if err := walk(); err != nil {
						return err
					}
				}
				end, endErr := decoder.Token()
				if endErr != nil || end != json.Delim(']') {
					return errors.New("invalid JSON array")
				}
			default:
				return errors.New("invalid JSON delimiter")
			}
		}
		return nil
	}
	if err := walk(); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func toContentResponse(detail ports.ContentDetail) contentResponse {
	return contentResponse{
		ContentID: detail.ContentID, TranslationID: detail.TranslationID,
		AccountID: detail.AccountID, LocaleID: detail.LocaleID,
		Kind: detail.Kind, ContentKey: detail.ContentKey,
		ContentStatus: detail.ContentStatus, ContentVersion: detail.ContentVersion,
		TranslationStatus:  detail.TranslationStatus,
		TranslationVersion: detail.TranslationVersion, Slug: detail.Slug,
		Path: detail.Path, Title: detail.Title, Excerpt: detail.Excerpt,
		Blocks: detail.Blocks,
		SEO: contentMetadataResponse{
			Title: detail.SEO.Title, Description: detail.SEO.Description,
			CanonicalURL: detail.SEO.CanonicalURL, Robots: detail.SEO.Robots,
			StructuredData: detail.SEO.StructuredData,
		},
		GEO: contentGeoRequest{Region: detail.GEO.Region, Locality: detail.GEO.Locality},
		AEO: contentAEORequest{Question: detail.AEO.Question, Answer: detail.AEO.Answer},
	}
}

func writeContentMutationError(ctx *gin.Context, err error) {
	status := nethttp.StatusUnprocessableEntity
	switch {
	case errors.Is(err, ports.ErrContentConflict):
		status = nethttp.StatusConflict
	case errors.Is(err, ports.ErrContentNotFound):
		status = nethttp.StatusNotFound
	case errors.Is(err, contentapp.ErrInvalidContentMutation),
		errors.Is(err, contentapp.ErrInvalidContent):
		status = nethttp.StatusUnprocessableEntity
	default:
		status = nethttp.StatusInternalServerError
	}
	writeContentProblem(ctx, status, err)
}

func writeContentProblem(ctx *gin.Context, status int, cause error) {
	writeContentProblemCode(
		ctx,
		status,
		"publishing_request_failed",
		"The publishing request could not be completed.",
		cause,
	)
}

func writeContentProblemCode(
	ctx *gin.Context,
	status int,
	code string,
	message string,
	cause error,
) {
	response.WriteProblem(
		ctx,
		status,
		platformmiddleware.RequestIDFromContext(ctx.Request.Context()),
		response.NewProblem(
			code,
			message,
			nil,
			cause,
		),
	)
}
