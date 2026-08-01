package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestM3A3ScheduleRequiresVersionAndIdempotency(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	schedule, err := NewPublicationSchedule(PublicationScheduleInput{
		ID:              "018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
		AccountID:       "018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
		TranslationID:   "018f1f64-7b2a-7c10-8bf2-3c40d5f67587",
		PublishAt:       now.Add(time.Hour),
		ExpectedVersion: 3,
		IdempotencyKey:  "request-2026-08-02-0001",
		State:           SchedulePending,
		FailureReason:   "",
	})
	require.NoError(t, err)
	require.Equal(t, SchedulePending, schedule.State)

	_, err = NewPublicationSchedule(PublicationScheduleInput{
		ID:              "id",
		TranslationID:   "translation",
		PublishAt:       now,
		ExpectedVersion: 0,
		IdempotencyKey:  "",
	})
	require.ErrorIs(t, err, ErrInvalidPublicationSchedule)
}
