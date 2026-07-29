package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeLocaleTagAcceptsArbitraryValidBCP47Tags(t *testing.T) {
	t.Parallel()

	for input, expected := range map[string]string{
		"th":      "th",
		"EN-us":   "en-us",
		"zh-Hant": "zh-hant",
		"ar-EG":   "ar-eg",
	} {
		actual, err := NormalizeLocaleTag(input)
		require.NoError(t, err, input)
		require.Equal(t, expected, actual)
	}
}

func TestNormalizeLocaleTagRejectsInvalidInternalIdentities(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"", "en_US", "und", "not a tag", "ไทย"} {
		_, err := NormalizeLocaleTag(input)
		require.ErrorIs(t, err, ErrInvalidLocaleTag, input)
	}
}
