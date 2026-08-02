package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestM3B5SEODefaultsNormalizeCanonicalAndRobots(t *testing.T) {
	t.Parallel()

	defaults, err := NewSEODefaults(SEODefaultsInput{
		ID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67580", AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67581", LocaleID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67582",
		Title: "  Default title ", Description: "Description", CanonicalBase: "https://www.example.test/", Robots: RobotsIndexFollow, Version: 1,
	})
	require.NoError(t, err)
	require.Equal(t, "Default title", defaults.Title)
	require.Equal(t, "https://www.example.test", defaults.CanonicalBase)

	_, err = NewSEODefaults(SEODefaultsInput{ID: defaults.ID, AccountID: defaults.AccountID, LocaleID: defaults.LocaleID, Title: "Title", CanonicalBase: "http://example.test", Robots: "all", Version: 1})
	require.ErrorIs(t, err, ErrInvalidSEODefaults)
}
