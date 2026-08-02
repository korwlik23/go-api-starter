package preview

import (
	"context"
	"time"

	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
)

type ExchangeDependencies struct {
	Repository ports.PreviewRepository
	Revisions  ports.RevisionRepository
	Codec      PreviewTokenCodec
	Now        func() time.Time
}

type Exchange struct {
	dependencies ExchangeDependencies
}

func NewExchange(dependencies ExchangeDependencies) (*Exchange, error) {
	if dependencies.Repository == nil || dependencies.Revisions == nil ||
		dependencies.Codec == nil || dependencies.Now == nil {
		return nil, ErrInvalidPreviewExchange
	}
	return &Exchange{dependencies: dependencies}, nil
}

type ExchangeResult struct {
	Revision  ports.RevisionView
	ExpiresAt time.Time
}

func (service *Exchange) Execute(ctx context.Context, rawToken string) (ExchangeResult, error) {
	if service == nil || ctx == nil || rawToken == "" {
		return ExchangeResult{}, ports.ErrPreviewNotFound
	}
	tokenID, ok := service.dependencies.Codec.Parse(rawToken)
	if !ok || tokenID == "" {
		return ExchangeResult{}, ports.ErrPreviewNotFound
	}
	record, err := service.dependencies.Repository.Find(ctx, tokenID)
	if err != nil || ports.ValidatePreviewToken(record) != nil || record.ID != tokenID ||
		record.Audience != PreviewAudiencePublic {
		return ExchangeResult{}, ports.ErrPreviewNotFound
	}
	now := service.dependencies.Now().UTC()
	if now.IsZero() || now.Before(record.CreatedAt) || !now.Before(record.ExpiresAt) ||
		!service.dependencies.Codec.Matches(
			PreviewPurpose, record.ID, record.UserID, rawToken,
			record.HashKeyVersion, record.TokenHash,
		) {
		return ExchangeResult{}, ports.ErrPreviewNotFound
	}
	if err := service.dependencies.Repository.Consume(ctx, record.ID, record.TokenHash, now); err != nil {
		return ExchangeResult{}, ports.ErrPreviewNotFound
	}
	view, err := service.dependencies.Revisions.Get(
		ctx, record.AccountID, record.ContentID, record.RevisionID,
	)
	if err != nil || view.AccountID != record.AccountID ||
		view.Revision.ID != record.RevisionID || view.Revision.ContentID != record.ContentID ||
		view.Revision.TranslationID != record.TranslationID || len(view.Snapshot) == 0 {
		return ExchangeResult{}, ports.ErrPreviewNotFound
	}
	return ExchangeResult{Revision: view, ExpiresAt: record.ExpiresAt}, nil
}
