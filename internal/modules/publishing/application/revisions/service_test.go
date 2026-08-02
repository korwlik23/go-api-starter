package revisions

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	"github.com/stretchr/testify/require"
)

const (
	revisionAccountID     = "018f1f64-7b2a-7c10-8bf2-3c40d5f67580"
	revisionContentID     = "018f1f64-7b2a-7c10-8bf2-3c40d5f67581"
	revisionTranslationID = "018f1f64-7b2a-7c10-8bf2-3c40d5f67582"
	revisionLocaleID      = "018f1f64-7b2a-7c10-8bf2-3c40d5f67583"
)

func TestM3A6CreateRevisionStoresHashedImmutableSnapshot(t *testing.T) {
	t.Parallel()

	repository := &revisionRepositoryFake{}
	service, err := NewService(RevisionDependencies{
		Repository: repository,
		Content:    &revisionContentRepositoryFake{},
		Auditor:    &revisionAuditorFake{},
		NewID:      sequenceRevisionID,
		Now:        fixedRevisionNow,
	})
	require.NoError(t, err)

	view, err := service.Create(context.Background(), CreateInput{
		AccountID: revisionAccountID, ActorUserID: revisionLocaleID,
		RequestID: "revision-create", ContentID: revisionContentID,
		TranslationID: revisionTranslationID, SourceVersion: 2,
		Snapshot: validRevisionSnapshot(),
	})
	require.NoError(t, err)
	require.NotEmpty(t, repository.created.Snapshot)
	require.Len(t, view.Revision.SnapshotHash, 64)
	require.Equal(t, view.Revision.SnapshotHash, repository.created.Revision.SnapshotHash)
}

func TestM3A6CompareReturnsDeterministicFieldChanges(t *testing.T) {
	t.Parallel()

	baseSnapshot := validRevisionSnapshot()
	targetSnapshot := validRevisionSnapshot()
	targetSnapshot.Title = "Updated home"
	baseRaw, baseHash, err := encodeSnapshot(baseSnapshot)
	require.NoError(t, err)
	targetRaw, targetHash, err := encodeSnapshot(targetSnapshot)
	require.NoError(t, err)
	repository := &revisionRepositoryFake{views: map[string]ports.RevisionView{
		"base": {
			AccountID: revisionAccountID,
			Revision:  publishingdomain.Revision{ID: "base", ContentID: revisionContentID, TranslationID: revisionTranslationID, SnapshotHash: baseHash, SourceVersion: 1, CreatedAt: fixedRevisionNow()},
			Snapshot:  baseRaw,
		},
		"target": {
			AccountID: revisionAccountID,
			Revision:  publishingdomain.Revision{ID: "target", ContentID: revisionContentID, TranslationID: revisionTranslationID, SnapshotHash: targetHash, SourceVersion: 2, CreatedAt: fixedRevisionNow()},
			Snapshot:  targetRaw,
		},
	}}
	service, err := NewService(RevisionDependencies{Repository: repository, Content: &revisionContentRepositoryFake{}, NewID: sequenceRevisionID, Now: fixedRevisionNow})
	require.NoError(t, err)

	result, err := service.Compare(context.Background(), CompareInput{AccountID: revisionAccountID, ContentID: revisionContentID, BaseRevisionID: "base", TargetRevisionID: "target"})
	require.NoError(t, err)
	require.Equal(t, []FieldChange{{Field: "title", From: "Home", To: "Updated home"}}, result.Changes)
}

func TestM3A6RollbackCreatesNewRevisionAndAuditsWithoutSnapshotPayload(t *testing.T) {
	t.Parallel()

	raw, hash, err := encodeSnapshot(validRevisionSnapshot())
	require.NoError(t, err)
	auditor := &revisionAuditorFake{}
	repository := &revisionRepositoryFake{views: map[string]ports.RevisionView{
		"source": {
			AccountID: revisionAccountID,
			Revision:  publishingdomain.Revision{ID: "source", ContentID: revisionContentID, TranslationID: revisionTranslationID, SnapshotHash: hash, SourceVersion: 1, CreatedAt: fixedRevisionNow()},
			Snapshot:  raw,
		},
	}}
	service, err := NewService(RevisionDependencies{
		Repository: repository, Content: &revisionContentRepositoryFake{detail: revisionDetail(2)},
		Auditor: auditor, NewID: sequenceRevisionID, Now: fixedRevisionNow,
	})
	require.NoError(t, err)

	result, err := service.Rollback(context.Background(), RollbackInput{
		AccountID: revisionAccountID, ActorUserID: revisionLocaleID, RequestID: "revision-rollback",
		ContentID: revisionContentID, TranslationID: revisionTranslationID, LocaleID: revisionLocaleID,
		RevisionID: "source", ExpectedVersion: 2,
	})
	require.NoError(t, err)
	require.Equal(t, int64(3), result.Revision.Revision.SourceVersion)
	require.Equal(t, "publishing.revision.rolled_back", auditor.event.Action)
	require.NotContains(t, auditor.event.Data, "snapshot")
	require.Equal(t, int64(3), repository.restored.TranslationVersion)
}

func validRevisionSnapshot() Snapshot {
	return Snapshot{
		Kind: publishingdomain.KindPage, ContentKey: "home", LocaleID: revisionLocaleID,
		Slug: "home", Path: "/home", Title: "Home",
		Blocks: []publishingdomain.Block{{Type: publishingdomain.BlockText, Data: map[string]any{"text": "Welcome"}}},
		Status: publishingdomain.StatusDraft,
	}
}

func revisionDetail(version int64) ports.ContentDetail {
	detail := ports.ContentDetail{
		ContentID: revisionContentID, TranslationID: revisionTranslationID,
		AccountID: revisionAccountID, LocaleID: revisionLocaleID,
		Kind: publishingdomain.KindPage, ContentKey: "home",
		ContentStatus: publishingdomain.StatusDraft, ContentVersion: version,
		TranslationStatus: publishingdomain.StatusDraft, TranslationVersion: version,
		Slug: "home", Path: "/home", Title: "Home", Blocks: validRevisionSnapshot().Blocks,
		UpdatedAt: fixedRevisionNow(),
	}
	return detail
}

func fixedRevisionNow() time.Time { return time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC) }

var revisionIDSequence int32

func sequenceRevisionID() (string, error) {
	sequence := atomic.AddInt32((*int32)(&revisionIDSequence), 1)
	return "018f1f64-7b2a-7c10-8bf2-3c40d5f6758" + string(rune('0'+sequence)), nil
}

type revisionRepositoryFake struct {
	created  ports.CreateRevisionCommand
	views    map[string]ports.RevisionView
	restored ports.ContentDetail
}

func (repository *revisionRepositoryFake) Create(_ context.Context, command ports.CreateRevisionCommand) error {
	repository.created = command
	if repository.views == nil {
		repository.views = map[string]ports.RevisionView{}
	}
	repository.views[command.Revision.ID] = ports.RevisionView{AccountID: command.AccountID, Revision: command.Revision, Snapshot: command.Snapshot}
	return nil
}

func (repository *revisionRepositoryFake) List(_ context.Context, _ ports.RevisionListQuery) (ports.RevisionPage, error) {
	return ports.RevisionPage{}, nil
}

func (repository *revisionRepositoryFake) Get(_ context.Context, _, _, revisionID string) (ports.RevisionView, error) {
	view, ok := repository.views[revisionID]
	if !ok {
		return ports.RevisionView{}, ports.ErrRevisionNotFound
	}
	return view, nil
}

func (repository *revisionRepositoryFake) RestoreContent(_ context.Context, command ports.RestoreContentCommand) (ports.ContentDetail, error) {
	repository.restored = command.Detail
	return command.Detail, nil
}

type revisionContentRepositoryFake struct{ detail ports.ContentDetail }

func (repository *revisionContentRepositoryFake) List(context.Context, ports.ContentListQuery) (ports.ContentListPage, error) {
	return ports.ContentListPage{}, nil
}
func (repository *revisionContentRepositoryFake) Get(context.Context, string, string, string) (ports.ContentDetail, error) {
	if repository.detail.ContentID == "" {
		return revisionDetail(2), nil
	}
	return repository.detail, nil
}
func (repository *revisionContentRepositoryFake) FindPublic(context.Context, string, string, string) (ports.ContentDetail, error) {
	return ports.ContentDetail{}, ports.ErrContentNotFound
}

type revisionAuditorFake struct {
	event RevisionAuditEvent
	err   error
}

func (auditor *revisionAuditorFake) RecordRevisionChange(_ context.Context, event RevisionAuditEvent) error {
	auditor.event = event
	return auditor.err
}
