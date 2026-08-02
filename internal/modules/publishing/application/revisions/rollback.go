package revisions

import (
	"context"
	"strings"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
)

func (service *Service) Rollback(
	ctx context.Context,
	input RollbackInput,
) (result RollbackResult, resultErr error) {
	if service == nil || ctx == nil ||
		strings.TrimSpace(input.AccountID) == "" ||
		strings.TrimSpace(input.ActorUserID) == "" ||
		strings.TrimSpace(input.RequestID) == "" ||
		strings.TrimSpace(input.ContentID) == "" ||
		strings.TrimSpace(input.TranslationID) == "" ||
		strings.TrimSpace(input.LocaleID) == "" ||
		strings.TrimSpace(input.RevisionID) == "" ||
		input.ExpectedVersion < 1 {
		return RollbackResult{}, ErrInvalidRevisionService
	}
	now := service.dependencies.Now().UTC()
	if now.IsZero() {
		return RollbackResult{}, ErrInvalidRevisionService
	}
	source, err := service.dependencies.Repository.Get(
		ctx, input.AccountID, input.ContentID, input.RevisionID,
	)
	if err != nil {
		return RollbackResult{}, err
	}
	if source.Revision.TranslationID != input.TranslationID {
		return RollbackResult{}, ports.ErrRevisionNotFound
	}
	snapshot, err := decodeSnapshot(source.Snapshot, source.Revision.SnapshotHash)
	if err != nil {
		return RollbackResult{}, err
	}
	current, err := service.dependencies.Content.Get(
		ctx, input.AccountID, input.ContentID, input.LocaleID,
	)
	if err != nil {
		return RollbackResult{}, err
	}
	detail := snapshotDetail(snapshot, current, now)
	if detail.TranslationVersion != input.ExpectedVersion+1 {
		return RollbackResult{}, ports.ErrRevisionConflict
	}
	raw, hash, err := encodeSnapshot(snapshot)
	if err != nil {
		return RollbackResult{}, err
	}
	newRevisionID, err := service.dependencies.NewID()
	if err != nil {
		return RollbackResult{}, err
	}
	revision, err := publishingdomain.NewRevision(publishingdomain.RevisionInput{
		ID: newRevisionID, ContentID: input.ContentID, TranslationID: input.TranslationID,
		AuthorID: input.ActorUserID, SnapshotHash: hash,
		ChangeSummary: input.ChangeSummary, SourceVersion: input.ExpectedVersion + 1,
		CreatedAt: now,
	})
	if err != nil {
		return RollbackResult{}, ErrInvalidRevisionService
	}
	operation := func(operationContext context.Context) error {
		restored, restoreErr := service.dependencies.Repository.RestoreContent(
			operationContext,
			ports.RestoreContentCommand{
				AccountID: input.AccountID, ContentID: input.ContentID,
				TranslationID: input.TranslationID, ExpectedVersion: input.ExpectedVersion,
				Detail: detail, UpdatedAt: now,
			},
		)
		if restoreErr != nil {
			return restoreErr
		}
		result.Content = restored
		if err := service.dependencies.Repository.Create(
			operationContext,
			ports.CreateRevisionCommand{AccountID: input.AccountID, Revision: revision, Snapshot: raw},
		); err != nil {
			return err
		}
		if service.dependencies.Auditor != nil {
			if err := service.dependencies.Auditor.RecordRevisionChange(
				operationContext,
				RevisionAuditEvent{
					ActorUserID: input.ActorUserID, AccountID: input.AccountID,
					RequestID: input.RequestID, Action: "publishing.revision.rolled_back",
					ContentID: input.ContentID, TranslationID: input.TranslationID,
					RevisionID: revision.ID, Version: revision.SourceVersion,
					Data: map[string]any{"source_revision_id": source.Revision.ID, "source_version": source.Revision.SourceVersion},
				},
			); err != nil {
				return err
			}
		}
		result.Revision = ports.RevisionView{AccountID: input.AccountID, Revision: revision, Snapshot: raw}
		return nil
	}
	resultErr = service.runMutation(ctx, operation)
	return result, resultErr
}
