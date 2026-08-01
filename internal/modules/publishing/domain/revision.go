package domain

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

var (
	ErrInvalidRevision  = errors.New("publishing: invalid revision")
	revisionHashPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

type Revision struct {
	ID            string
	ContentID     string
	TranslationID string
	AuthorID      string
	SnapshotHash  string
	ChangeSummary string
	SourceVersion int64
	CreatedAt     time.Time
}

type RevisionInput struct {
	ID            string
	ContentID     string
	TranslationID string
	AuthorID      string
	SnapshotHash  string
	ChangeSummary string
	SourceVersion int64
	CreatedAt     time.Time
}

func NewRevision(input RevisionInput) (Revision, error) {
	if !validIdentity(input.ID) ||
		!validIdentity(input.ContentID) ||
		!validIdentity(input.TranslationID) ||
		!validIdentity(input.AuthorID) ||
		!revisionHashPattern.MatchString(input.SnapshotHash) ||
		len(input.ChangeSummary) > 1000 ||
		input.SourceVersion <= 0 || input.CreatedAt.IsZero() {
		return Revision{}, ErrInvalidRevision
	}
	return Revision{
		ID:            strings.TrimSpace(input.ID),
		ContentID:     strings.TrimSpace(input.ContentID),
		TranslationID: strings.TrimSpace(input.TranslationID),
		AuthorID:      strings.TrimSpace(input.AuthorID),
		SnapshotHash:  input.SnapshotHash,
		ChangeSummary: input.ChangeSummary,
		SourceVersion: input.SourceVersion,
		CreatedAt:     input.CreatedAt.UTC(),
	}, nil
}
