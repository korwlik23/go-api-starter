package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestM3B5RedirectAcceptsSafeSameSiteDestination(t *testing.T) {
	t.Parallel()

	redirect, err := NewRedirect(RedirectInput{
		ID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67580", AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67581",
		LocaleID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67582", SourcePath: "/old", DestinationPath: "/new",
		StatusCode: StatusMovedPermanently, Enabled: true, Version: 1,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
	require.NoError(t, err)
	require.Equal(t, "/old", redirect.SourcePath)
	require.Equal(t, StatusMovedPermanently, redirect.StatusCode)
}

func TestM3B5RedirectRejectsOpenRedirectLoopAndUnsupportedStatus(t *testing.T) {
	t.Parallel()
	base := RedirectInput{ID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67580", AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67581", LocaleID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67582", SourcePath: "/old", DestinationPath: "/new", StatusCode: StatusFound, Version: 1}
	_, err := NewRedirect(base)
	require.NoError(t, err)
	base.DestinationPath = "https://evil.example/steal"
	_, err = NewRedirect(base)
	require.ErrorIs(t, err, ErrInvalidRedirect)
	base.DestinationPath = "/old"
	_, err = NewRedirect(base)
	require.ErrorIs(t, err, ErrRedirectLoop)
	base.DestinationPath = "/new"
	base.StatusCode = 200
	_, err = NewRedirect(base)
	require.ErrorIs(t, err, ErrInvalidRedirect)
}
