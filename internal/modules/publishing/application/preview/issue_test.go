package preview

import (
	"context"
	"testing"
	"time"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/crypto"
	"github.com/stretchr/testify/require"
)

const (
	previewAccountID     = "018f1f64-7b2a-7c10-8bf2-3c40d5f67580"
	previewUserID        = "018f1f64-7b2a-7c10-8bf2-3c40d5f67581"
	previewContentID     = "018f1f64-7b2a-7c10-8bf2-3c40d5f67582"
	previewTranslationID = "018f1f64-7b2a-7c10-8bf2-3c40d5f67583"
	previewRevisionID    = "018f1f64-7b2a-7c10-8bf2-3c40d5f67584"
	previewTokenID       = "018f1f64-7b2a-7c10-8bf2-3c40d5f67585"
)

func TestM3C2IssueBindsAuthenticatedUserContentAndRevision(t *testing.T) {
	t.Parallel()

	repository := &previewRepositoryFake{}
	issue, err := NewIssue(IssueDependencies{
		Repository: repository,
		Revisions:  &previewRevisionRepositoryFake{view: previewRevisionView()},
		Codec:      &previewCodecFake{match: true},
		NewID:      func() (string, error) { return previewTokenID, nil },
		Now:        func() time.Time { return time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC) },
		TTL:        5 * time.Minute,
	})
	require.NoError(t, err)

	result, err := issue.Execute(context.Background(), IssueInput{
		AccountID: previewAccountID, UserID: previewUserID,
		ContentID: previewContentID, TranslationID: previewTranslationID,
		RevisionID: previewRevisionID,
	})
	require.NoError(t, err)
	require.NotEmpty(t, result.RawToken)
	require.Equal(t, previewTokenID, repository.created.ID)
	require.Equal(t, previewUserID, repository.created.UserID)
	require.Equal(t, previewContentID, repository.created.ContentID)
	require.Equal(t, previewRevisionID, repository.created.RevisionID)
	require.Equal(t, PreviewAudiencePublic, repository.created.Audience)
	require.Equal(t, time.Date(2026, 8, 2, 12, 5, 0, 0, time.UTC), repository.created.ExpiresAt)
}

func TestM3C2IssueRejectsRevisionFromAnotherAccount(t *testing.T) {
	t.Parallel()

	view := previewRevisionView()
	view.AccountID = "018f1f64-7b2a-7c10-8bf2-3c40d5f67586"
	issue, err := NewIssue(IssueDependencies{
		Repository: &previewRepositoryFake{}, Revisions: &previewRevisionRepositoryFake{view: view},
		Codec: &previewCodecFake{match: true}, NewID: func() (string, error) { return previewTokenID, nil },
		Now: func() time.Time { return time.Now().UTC() }, TTL: time.Minute,
	})
	require.NoError(t, err)

	_, err = issue.Execute(context.Background(), IssueInput{
		AccountID: previewAccountID, UserID: previewUserID,
		ContentID: previewContentID, TranslationID: previewTranslationID, RevisionID: previewRevisionID,
	})
	require.ErrorIs(t, err, ports.ErrPreviewNotFound)
}

type previewRepositoryFake struct {
	created    ports.PreviewToken
	record     ports.PreviewToken
	findErr    error
	consumeErr error
	consumed   bool
}

func (repository *previewRepositoryFake) Create(_ context.Context, token ports.PreviewToken) error {
	repository.created = token
	repository.record = token
	return nil
}

func (repository *previewRepositoryFake) Find(_ context.Context, _ string) (ports.PreviewToken, error) {
	if repository.findErr != nil {
		return ports.PreviewToken{}, repository.findErr
	}
	return repository.record, nil
}

func (repository *previewRepositoryFake) Consume(_ context.Context, _ string, _ [32]byte, _ time.Time) error {
	if repository.consumeErr != nil {
		return repository.consumeErr
	}
	if repository.consumed {
		return ports.ErrPreviewConflict
	}
	repository.consumed = true
	return nil
}

type previewRevisionRepositoryFake struct{ view ports.RevisionView }

func (repository *previewRevisionRepositoryFake) Create(context.Context, ports.CreateRevisionCommand) error {
	return nil
}
func (repository *previewRevisionRepositoryFake) List(context.Context, ports.RevisionListQuery) (ports.RevisionPage, error) {
	return ports.RevisionPage{}, nil
}
func (repository *previewRevisionRepositoryFake) Get(context.Context, string, string, string) (ports.RevisionView, error) {
	return repository.view, nil
}
func (repository *previewRevisionRepositoryFake) RestoreContent(context.Context, ports.RestoreContentCommand) (ports.ContentDetail, error) {
	return ports.ContentDetail{}, nil
}

type previewCodecFake struct{ match bool }

func (*previewCodecFake) Generate(string, string, string) (crypto.BoundToken, error) {
	return crypto.BoundToken{Raw: previewTokenID + ".raw", Hash: [32]byte{1}, HashVersion: 1, HashKeyVersion: 1}, nil
}
func (*previewCodecFake) Parse(string) (string, bool) { return previewTokenID, true }
func (codec *previewCodecFake) Matches(string, string, string, string, int64, [32]byte) bool {
	return codec.match
}

func previewRevisionView() ports.RevisionView {
	return ports.RevisionView{
		AccountID: previewAccountID,
		Revision: publishingdomain.Revision{
			ID: previewRevisionID, ContentID: previewContentID,
			TranslationID: previewTranslationID, AuthorID: previewUserID,
			SnapshotHash:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			SourceVersion: 3, CreatedAt: time.Date(2026, 8, 2, 11, 0, 0, 0, time.UTC),
		},
		Snapshot: []byte(`{"kind":"page","content_key":"home","locale_id":"018f1f64-7b2a-7c10-8bf2-3c40d5f67586","slug":"home","path":"/home","title":"Home","blocks":[{"type":"text","data":{"text":"Home"}}],"status":"draft"}`),
	}
}

var _ ports.PreviewRepository = (*previewRepositoryFake)(nil)
var _ ports.RevisionRepository = (*previewRevisionRepositoryFake)(nil)
var _ PreviewTokenCodec = (*previewCodecFake)(nil)
