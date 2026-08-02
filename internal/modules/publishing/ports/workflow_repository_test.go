package ports

import (
	"testing"
	"time"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/stretchr/testify/require"
)

func TestM3A7WorkflowEventRequiresExplicitTransitionIdentity(t *testing.T) {
	t.Parallel()

	event := WorkflowEvent{
		ID:            "018f1f64-7b2a-7c10-8bf2-3c40d5f67580",
		AccountID:     "018f1f64-7b2a-7c10-8bf2-3c40d5f67581",
		ContentID:     "018f1f64-7b2a-7c10-8bf2-3c40d5f67582",
		TranslationID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67583",
		ActorID:       "018f1f64-7b2a-7c10-8bf2-3c40d5f67584",
		FromStatus:    publishingdomain.StatusDraft, ToStatus: publishingdomain.StatusReview,
		RequestID: "request-workflow", OccurredAt: time.Now().UTC(),
	}
	require.NoError(t, ValidateWorkflowEvent(event))
	event.ToStatus = publishingdomain.StatusDraft
	require.ErrorIs(t, ValidateWorkflowEvent(event), ErrInvalidWorkflowRepository)
}
