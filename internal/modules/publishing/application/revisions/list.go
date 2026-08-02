package revisions

import (
	"context"
	"strings"

	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
)

func (service *Service) List(
	ctx context.Context,
	input ListInput,
) (ports.RevisionPage, error) {
	if service == nil || ctx == nil || strings.TrimSpace(input.AccountID) == "" {
		return ports.RevisionPage{}, ErrInvalidRevisionService
	}
	query := ports.RevisionListQuery{
		AccountID: input.AccountID, ContentID: input.ContentID,
		TranslationID: input.TranslationID, After: input.After, Limit: input.Limit,
	}
	if err := ports.ValidateRevisionListQuery(query); err != nil {
		return ports.RevisionPage{}, err
	}
	page, err := service.dependencies.Repository.List(ctx, query)
	if err != nil {
		return ports.RevisionPage{}, err
	}
	for index := range page.Items {
		if _, err := decodeSnapshot(page.Items[index].Snapshot, page.Items[index].Revision.SnapshotHash); err != nil {
			return ports.RevisionPage{}, err
		}
	}
	return page, nil
}
