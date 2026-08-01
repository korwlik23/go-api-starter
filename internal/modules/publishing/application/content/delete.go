package content

import (
	"context"
	"strings"

	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
)

func (service *ContentMutations) Delete(
	ctx context.Context,
	input DeleteInput,
) error {
	if service == nil || ctx == nil ||
		strings.TrimSpace(input.AccountID) == "" ||
		strings.TrimSpace(input.ActorUserID) == "" ||
		strings.TrimSpace(input.RequestID) == "" ||
		strings.TrimSpace(input.ContentID) == "" ||
		strings.TrimSpace(input.TranslationID) == "" ||
		strings.TrimSpace(input.LocaleID) == "" ||
		input.ExpectedVersion < 1 {
		return ErrInvalidContentMutation
	}
	now := service.dependencies.Now().UTC()
	if now.IsZero() {
		return ErrInvalidContentMutation
	}
	return service.runMutation(ctx, func(operationContext context.Context) error {
		if err := service.dependencies.Repository.SoftDelete(
			operationContext,
			ports.DeleteContentCommand{
				AccountID: input.AccountID, ContentID: input.ContentID,
				TranslationID: input.TranslationID, LocaleID: input.LocaleID,
				ExpectedVersion: input.ExpectedVersion, DeletedAt: now,
			},
		); err != nil {
			return err
		}
		return service.dependencies.Auditor.RecordContentChange(
			operationContext,
			ContentAuditEvent{
				ActorUserID: input.ActorUserID, AccountID: input.AccountID,
				RequestID: input.RequestID, Action: "publishing.content.deleted",
				ContentID: input.ContentID, TranslationID: input.TranslationID,
				Version: input.ExpectedVersion + 1,
				Data:    map[string]any{"soft_deleted": true},
			},
		)
	})
}
