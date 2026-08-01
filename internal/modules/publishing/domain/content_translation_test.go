package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestM3A3TranslationNormalizesLocaleAndPath(t *testing.T) {
	t.Parallel()

	translation, err := NewContentTranslation(ContentTranslationInput{
		ID:        "018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
		AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
		ContentID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67587",
		Locale:    "TH-th",
		Slug:      "  Hello-World  ",
		Path:      "/docs/Hello-World",
		Title:     "Hello world",
		Status:    StatusDraft,
		Version:   1,
	})
	require.NoError(t, err)
	require.Equal(t, "th-th", translation.Locale)
	require.Equal(t, "hello-world", translation.Slug)
	require.Equal(t, "/docs/hello-world", translation.Path)
}

func TestM3A3TranslationRejectsTraversalAndInvalidLocale(t *testing.T) {
	t.Parallel()

	base := ContentTranslationInput{
		ID:        "018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
		AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
		ContentID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67587",
		Locale:    "en",
		Slug:      "hello",
		Path:      "/hello",
		Title:     "Hello",
		Status:    StatusDraft,
		Version:   1,
	}
	for _, mutate := range []func(*ContentTranslationInput){
		func(input *ContentTranslationInput) { input.Locale = "../../etc/passwd" },
		func(input *ContentTranslationInput) { input.Slug = "../hello" },
		func(input *ContentTranslationInput) { input.Path = "/docs//hello" },
		func(input *ContentTranslationInput) { input.Title = "  padded title" },
	} {
		candidate := base
		mutate(&candidate)
		_, err := NewContentTranslation(candidate)
		require.ErrorIs(t, err, ErrInvalidContentTranslation)
	}
}
