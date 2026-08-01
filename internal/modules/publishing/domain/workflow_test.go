package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestM3A3WorkflowAllowsOnlyContractTransitions(t *testing.T) {
	t.Parallel()

	allowed := []struct {
		from Status
		to   Status
	}{
		{StatusDraft, StatusReview},
		{StatusReview, StatusPublished},
		{StatusReview, StatusScheduled},
		{StatusDraft, StatusArchived},
		{StatusReview, StatusArchived},
		{StatusPublished, StatusArchived},
		{StatusScheduled, StatusPublished},
	}
	for _, transition := range allowed {
		require.NoError(t, ValidateTransition(transition.from, transition.to))
	}
	require.ErrorIs(
		t,
		ValidateTransition(StatusPublished, StatusDraft),
		ErrInvalidWorkflowTransition,
	)
}
