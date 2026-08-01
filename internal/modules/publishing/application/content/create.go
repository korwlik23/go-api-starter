package content

import (
	"context"
	"strings"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
)

func (service *ContentMutations) Create(
	ctx context.Context,
	input CreateInput,
) (result ports.ContentDetail, resultErr error) {
	if service == nil || ctx == nil ||
		strings.TrimSpace(input.AccountID) == "" ||
		strings.TrimSpace(input.ActorUserID) == "" ||
		strings.TrimSpace(input.RequestID) == "" {
		return ports.ContentDetail{}, ErrInvalidContentMutation
	}
	now := service.dependencies.Now().UTC()
	if now.IsZero() {
		return ports.ContentDetail{}, ErrInvalidContentMutation
	}
	itemID, err := service.dependencies.NewID()
	if err != nil {
		return ports.ContentDetail{}, err
	}
	translationID, err := service.dependencies.NewID()
	if err != nil {
		return ports.ContentDetail{}, err
	}
	item, err := publishingdomain.NewContentItem(publishingdomain.ContentItemInput{
		ID: itemID, AccountID: input.AccountID, Kind: input.Kind,
		ContentKey: input.ContentKey, Status: publishingdomain.StatusDraft,
		Version: 1, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return ports.ContentDetail{}, ErrInvalidContentMutation
	}
	translation, err := publishingdomain.NewContentTranslation(publishingdomain.ContentTranslationInput{
		ID: translationID, AccountID: input.AccountID, ContentID: itemID,
		Locale: input.Locale, Slug: input.Slug, Path: input.Path,
		Title: input.Title, Excerpt: input.Excerpt, Blocks: input.Blocks,
		SEO: input.SEO, GEO: input.GEO, AEO: input.AEO,
		Status: publishingdomain.StatusDraft, Version: 1,
		CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return ports.ContentDetail{}, ErrInvalidContentMutation
	}
	if err := Validate(ContentValidationInput{
		Locale: input.Locale, Slug: input.Slug, Path: input.Path,
		Title: input.Title, Excerpt: input.Excerpt, Blocks: input.Blocks,
		SEO: input.SEO, GEO: input.GEO, AEO: input.AEO,
	}); err != nil {
		return ports.ContentDetail{}, err
	}
	operation := func(operationContext context.Context) error {
		result, err = service.dependencies.Repository.Create(
			operationContext,
			ports.CreateContentCommand{Item: item, Translation: translation},
		)
		if err != nil {
			return err
		}
		return service.dependencies.Auditor.RecordContentChange(
			operationContext,
			ContentAuditEvent{
				ActorUserID: input.ActorUserID, AccountID: input.AccountID,
				RequestID: input.RequestID, Action: "publishing.content.created",
				ContentID: item.ID, TranslationID: translation.ID, Version: 1,
				Data: map[string]any{"kind": string(item.Kind), "status": string(item.Status)},
			},
		)
	}
	resultErr = service.runMutation(ctx, operation)
	return result, resultErr
}
