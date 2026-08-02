package preview

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/crypto"
)

const (
	PreviewPurpose        = "publishing.preview.v1"
	PreviewAudiencePublic = "public"
	DefaultPreviewTTL     = 5 * time.Minute
	MaximumPreviewTTL     = 15 * time.Minute
)

var (
	ErrInvalidPreviewIssue    = errors.New("invalid preview issue")
	ErrInvalidPreviewExchange = errors.New("invalid preview exchange")
)

type PreviewTokenCodec interface {
	Generate(string, string, string) (crypto.BoundToken, error)
	Parse(string) (string, bool)
	Matches(string, string, string, string, int64, [32]byte) bool
}

type IssueDependencies struct {
	Repository ports.PreviewRepository
	Revisions  ports.RevisionRepository
	Codec      PreviewTokenCodec
	NewID      func() (string, error)
	Now        func() time.Time
	TTL        time.Duration
}

type Issue struct {
	dependencies IssueDependencies
}

func NewIssue(dependencies IssueDependencies) (*Issue, error) {
	if dependencies.Repository == nil || dependencies.Revisions == nil ||
		dependencies.Codec == nil || dependencies.NewID == nil || dependencies.Now == nil ||
		dependencies.TTL <= 0 || dependencies.TTL > MaximumPreviewTTL {
		return nil, ErrInvalidPreviewIssue
	}
	return &Issue{dependencies: dependencies}, nil
}

type IssueInput struct {
	AccountID     string
	UserID        string
	ContentID     string
	TranslationID string
	RevisionID    string
}

type IssueResult struct {
	RawToken   string
	TokenID    string
	RevisionID string
	ExpiresAt  time.Time
}

func (service *Issue) Execute(ctx context.Context, input IssueInput) (IssueResult, error) {
	if service == nil || ctx == nil || !validPreviewIDs(
		input.AccountID, input.UserID, input.ContentID, input.TranslationID, input.RevisionID,
	) {
		return IssueResult{}, ErrInvalidPreviewIssue
	}
	view, err := service.dependencies.Revisions.Get(
		ctx, input.AccountID, input.ContentID, input.RevisionID,
	)
	if err != nil || view.AccountID != input.AccountID ||
		view.Revision.ContentID != input.ContentID ||
		view.Revision.TranslationID != input.TranslationID ||
		view.Revision.ID != input.RevisionID || len(view.Snapshot) == 0 {
		return IssueResult{}, ports.ErrPreviewNotFound
	}
	now := service.dependencies.Now().UTC()
	if now.IsZero() {
		return IssueResult{}, ErrInvalidPreviewIssue
	}
	tokenID, err := service.dependencies.NewID()
	if err != nil || !validPreviewID(tokenID) {
		return IssueResult{}, ErrInvalidPreviewIssue
	}
	token, err := service.dependencies.Codec.Generate(PreviewPurpose, tokenID, input.UserID)
	if err != nil || token.Raw == "" || token.Hash == [32]byte{} || token.HashVersion < 1 || token.HashKeyVersion < 1 {
		return IssueResult{}, ErrInvalidPreviewIssue
	}
	expiresAt := now.Add(service.dependencies.TTL).UTC()
	record := ports.PreviewToken{
		ID: tokenID, UserID: input.UserID, AccountID: input.AccountID,
		ContentID: input.ContentID, TranslationID: input.TranslationID,
		RevisionID: input.RevisionID, TokenHash: token.Hash,
		HashVersion: token.HashVersion, HashKeyVersion: token.HashKeyVersion,
		Audience: PreviewAudiencePublic, ExpiresAt: expiresAt, CreatedAt: now,
	}
	if err := ports.ValidatePreviewToken(record); err != nil {
		return IssueResult{}, ErrInvalidPreviewIssue
	}
	if err := service.dependencies.Repository.Create(ctx, record); err != nil {
		return IssueResult{}, err
	}
	return IssueResult{RawToken: token.Raw, TokenID: tokenID, RevisionID: input.RevisionID, ExpiresAt: expiresAt}, nil
}

func validPreviewIDs(values ...string) bool {
	for _, value := range values {
		if !validPreviewID(value) {
			return false
		}
	}
	return true
}

func validPreviewID(value string) bool {
	return strings.TrimSpace(value) != "" && len(value) <= 128 && !strings.ContainsAny(value, "/\\")
}
