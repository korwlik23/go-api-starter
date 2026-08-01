package content

import (
	"context"
	"strings"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
)

func (service *ContentMutations) Update(
	ctx context.Context,
	input UpdateInput,
) (result ports.ContentDetail, resultErr error) {
	if service == nil || ctx == nil ||
		strings.TrimSpace(input.AccountID) == "" ||
		strings.TrimSpace(input.ActorUserID) == "" ||
		strings.TrimSpace(input.RequestID) == "" ||
		strings.TrimSpace(input.ContentID) == "" ||
		strings.TrimSpace(input.TranslationID) == "" ||
		input.ExpectedVersion < 1 {
		return ports.ContentDetail{}, ErrInvalidContentMutation
	}
	now := service.dependencies.Now().UTC()
	if now.IsZero() {
		return ports.ContentDetail{}, ErrInvalidContentMutation
	}
	translation, err := publishingdomain.NewContentTranslation(publishingdomain.ContentTranslationInput{
		ID: input.TranslationID, AccountID: input.AccountID, ContentID: input.ContentID,
		Locale: input.Locale, Slug: input.Slug, Path: input.Path, Title: input.Title,
		Excerpt: input.Excerpt, Blocks: input.Blocks, SEO: input.SEO, GEO: input.GEO,
		AEO: input.AEO, Status: publishingdomain.StatusDraft,
		Version: input.ExpectedVersion + 1, UpdatedAt: now,
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
		result, err = service.dependencies.Repository.Update(
			operationContext,
			ports.UpdateContentCommand{
				AccountID: input.AccountID, ContentID: input.ContentID,
				TranslationID: input.TranslationID, LocaleID: input.Locale,
				ExpectedVersion: input.ExpectedVersion, Translation: translation,
			},
		)
		if err != nil {
			return err
		}
		version := result.TranslationVersion
		if version < 1 {
			version = input.ExpectedVersion + 1
		}
		return service.dependencies.Auditor.RecordContentChange(
			operationContext,
			ContentAuditEvent{
				ActorUserID: input.ActorUserID, AccountID: input.AccountID,
				RequestID: input.RequestID, Action: "publishing.content.updated",
				ContentID: input.ContentID, TranslationID: input.TranslationID,
				Version: version,
				Data:    map[string]any{"status": string(translation.Status)},
			},
		)
	}
	resultErr = service.runMutation(ctx, operation)
	return result, resultErr
}
