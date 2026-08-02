package preview

import (
	"context"
	"testing"
	"time"

	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	"github.com/stretchr/testify/require"
)

func TestM3C2ExchangeConsumesPreviewExactlyOnce(t *testing.T) {
	t.Parallel()

	repository := &previewRepositoryFake{record: ports.PreviewToken{
		ID: previewTokenID, UserID: previewUserID, AccountID: previewAccountID,
		ContentID: previewContentID, TranslationID: previewTranslationID, RevisionID: previewRevisionID,
		TokenHash: [32]byte{1}, HashVersion: 1, HashKeyVersion: 1,
		Audience:  PreviewAudiencePublic,
		CreatedAt: time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC),
		ExpiresAt: time.Date(2026, 8, 2, 12, 5, 0, 0, time.UTC),
	}}
	exchange, err := NewExchange(ExchangeDependencies{
		Repository: repository, Revisions: &previewRevisionRepositoryFake{view: previewRevisionView()},
		Codec: &previewCodecFake{match: true}, Now: func() time.Time { return time.Date(2026, 8, 2, 12, 1, 0, 0, time.UTC) },
	})
	require.NoError(t, err)

	result, err := exchange.Execute(context.Background(), previewTokenID+".raw")
	require.NoError(t, err)
	require.True(t, repository.consumed)
	require.Equal(t, previewRevisionID, result.Revision.Revision.ID)

	_, err = exchange.Execute(context.Background(), previewTokenID+".raw")
	require.ErrorIs(t, err, ports.ErrPreviewNotFound)
}

func TestM3C2ExchangeMapsTamperExpiryAudienceAndUnknownTokenToSameNotFound(t *testing.T) {
	t.Parallel()

	base := ports.PreviewToken{
		ID: previewTokenID, UserID: previewUserID, AccountID: previewAccountID,
		ContentID: previewContentID, TranslationID: previewTranslationID, RevisionID: previewRevisionID,
		TokenHash: [32]byte{1}, HashVersion: 1, HashKeyVersion: 1, Audience: PreviewAudiencePublic,
		CreatedAt: time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC),
		ExpiresAt: time.Date(2026, 8, 2, 12, 5, 0, 0, time.UTC),
	}
	for _, test := range []struct {
		name   string
		mutate func(*ports.PreviewToken, *previewRepositoryFake, *previewCodecFake, *time.Time)
	}{
		{name: "expired", mutate: func(record *ports.PreviewToken, _ *previewRepositoryFake, _ *previewCodecFake, now *time.Time) {
			*now = record.ExpiresAt.Add(time.Second)
		}},
		{name: "wrong audience", mutate: func(record *ports.PreviewToken, _ *previewRepositoryFake, _ *previewCodecFake, _ *time.Time) {
			record.Audience = "admin"
		}},
		{name: "tampered", mutate: func(_ *ports.PreviewToken, _ *previewRepositoryFake, codec *previewCodecFake, _ *time.Time) {
			codec.match = false
		}},
		{name: "unknown", mutate: func(_ *ports.PreviewToken, repository *previewRepositoryFake, _ *previewCodecFake, _ *time.Time) {
			repository.findErr = ports.ErrPreviewNotFound
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			record := base
			now := time.Date(2026, 8, 2, 12, 1, 0, 0, time.UTC)
			repository := &previewRepositoryFake{record: record}
			codec := &previewCodecFake{match: true}
			test.mutate(&record, repository, codec, &now)
			repository.record = record
			exchange, err := NewExchange(ExchangeDependencies{
				Repository: repository, Revisions: &previewRevisionRepositoryFake{view: previewRevisionView()},
				Codec: codec, Now: func() time.Time { return now },
			})
			require.NoError(t, err)
			_, err = exchange.Execute(context.Background(), previewTokenID+".raw")
			require.ErrorIs(t, err, ports.ErrPreviewNotFound)
		})
	}
}
