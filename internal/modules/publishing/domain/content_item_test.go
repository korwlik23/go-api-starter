package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestM3A3ContentItemAcceptsPageAndPostValues(t *testing.T) {
	t.Parallel()

	item, err := NewContentItem(ContentItemInput{
		ID:         "018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
		AccountID:  "018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
		Kind:       KindPage,
		ContentKey: "about-us",
		Status:     StatusDraft,
		Version:    1,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	})
	require.NoError(t, err)
	require.Equal(t, KindPage, item.Kind)
	require.Equal(t, StatusDraft, item.Status)
	require.Equal(t, int64(1), item.Version)
}

func TestM3A3ContentItemRejectsUnsafeIdentityAndVersion(t *testing.T) {
	t.Parallel()

	base := ContentItemInput{
		ID:         "018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
		AccountID:  "018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
		Kind:       KindPost,
		ContentKey: "news",
		Status:     StatusDraft,
		Version:    1,
	}
	for _, mutate := range []func(*ContentItemInput){
		func(input *ContentItemInput) { input.Kind = Kind("landing") },
		func(input *ContentItemInput) { input.ContentKey = "../private" },
		func(input *ContentItemInput) { input.Version = 0 },
		func(input *ContentItemInput) { input.AccountID = "" },
	} {
		candidate := base
		mutate(&candidate)
		_, err := NewContentItem(candidate)
		require.ErrorIs(t, err, ErrInvalidContentItem)
	}
}
