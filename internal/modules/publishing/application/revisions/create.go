package revisions

import (
	"context"
	"strings"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
)

func (service *Service) Create(
	ctx context.Context,
	input CreateInput,
) (ports.RevisionView, error) {
	if service == nil || ctx == nil ||
		strings.TrimSpace(input.AccountID) == "" ||
		strings.TrimSpace(input.ActorUserID) == "" ||
		strings.TrimSpace(input.RequestID) == "" ||
		strings.TrimSpace(input.ContentID) == "" ||
		strings.TrimSpace(input.TranslationID) == "" ||
		input.SourceVersion < 1 {
		return ports.RevisionView{}, ErrInvalidRevisionService
	}
	now := service.dependencies.Now().UTC()
	if now.IsZero() {
		return ports.RevisionView{}, ErrInvalidRevisionService
	}
	raw, hash, err := encodeSnapshot(input.Snapshot)
	if err != nil {
		return ports.RevisionView{}, err
	}
	revisionID, err := service.dependencies.NewID()
	if err != nil {
		return ports.RevisionView{}, err
	}
	revision, err := publishingdomain.NewRevision(publishingdomain.RevisionInput{
		ID: revisionID, ContentID: input.ContentID, TranslationID: input.TranslationID,
		AuthorID: input.ActorUserID, SnapshotHash: hash,
		ChangeSummary: input.ChangeSummary, SourceVersion: input.SourceVersion,
		CreatedAt: now,
	})
	if err != nil {
		return ports.RevisionView{}, ErrInvalidRevisionService
	}
	view := ports.RevisionView{AccountID: input.AccountID, Revision: revision, Snapshot: raw}
	operation := func(operationContext context.Context) error {
		if err := service.dependencies.Repository.Create(
			operationContext,
			ports.CreateRevisionCommand{AccountID: input.AccountID, Revision: revision, Snapshot: raw},
		); err != nil {
			return err
		}
		if service.dependencies.Auditor == nil {
			return nil
		}
		return service.dependencies.Auditor.RecordRevisionChange(
			operationContext,
			RevisionAuditEvent{
				ActorUserID: input.ActorUserID, AccountID: input.AccountID,
				RequestID: input.RequestID, Action: "publishing.revision.created",
				ContentID: input.ContentID, TranslationID: input.TranslationID,
				RevisionID: revision.ID, Version: revision.SourceVersion,
				Data: map[string]any{"snapshot_hash": revision.SnapshotHash},
			},
		)
	}
	if err := service.runMutation(ctx, operation); err != nil {
		return ports.RevisionView{}, err
	}
	return view, nil
}
