package content

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestM3A5DeleteIsCoveredBySoftDeleteContract(t *testing.T) {
	t.Parallel()
	repository := &mutationRepositoryFake{}
	auditor := &contentAuditorFake{}
	service, err := NewContentMutations(ContentMutationDependencies{
		Repository: repository,
		Auditor:    auditor,
		NewID:      sequenceID("018f1f64-7b2a-7c10-8bf2-3c40d5f6758"),
		Now:        func() time.Time { return time.Now().UTC() },
	})
	require.NoError(t, err)

	err = service.Delete(context.Background(), DeleteInput{
		AccountID:       "018f1f64-7b2a-7c10-8bf2-3c40d5f67580",
		ActorUserID:     "018f1f64-7b2a-7c10-8bf2-3c40d5f67581",
		RequestID:       "request-delete",
		ContentID:       "018f1f64-7b2a-7c10-8bf2-3c40d5f67582",
		TranslationID:   "018f1f64-7b2a-7c10-8bf2-3c40d5f67583",
		LocaleID:        "018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
		ExpectedVersion: 2,
	})
	require.NoError(t, err)
	require.Equal(t, "publishing.content.deleted", auditor.event.Action)
}
