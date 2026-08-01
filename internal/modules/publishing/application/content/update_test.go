package content

import (
	"context"
	"testing"
	"time"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/stretchr/testify/require"
)

func TestM3A5UpdateIsCoveredByCreateMutationContract(t *testing.T) {
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

	_, err = service.Update(context.Background(), UpdateInput{
		AccountID:     "018f1f64-7b2a-7c10-8bf2-3c40d5f67580",
		ActorUserID:   "018f1f64-7b2a-7c10-8bf2-3c40d5f67581",
		RequestID:     "request-update",
		ContentID:     "018f1f64-7b2a-7c10-8bf2-3c40d5f67582",
		TranslationID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67583",
		Locale:        "en", Slug: "news", Path: "/news", Title: "News",
		ExpectedVersion: 2,
		Blocks:          []publishingdomain.Block{{Type: publishingdomain.BlockText, Data: map[string]any{"text": "News"}}},
	})
	require.NoError(t, err)
	require.Equal(t, "publishing.content.updated", auditor.event.Action)
}
