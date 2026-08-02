package revisions

import (
	"context"
	"errors"
	"reflect"
	"strings"
)

func (service *Service) Compare(
	ctx context.Context,
	input CompareInput,
) (CompareResult, error) {
	if service == nil || ctx == nil || strings.TrimSpace(input.AccountID) == "" ||
		strings.TrimSpace(input.ContentID) == "" ||
		strings.TrimSpace(input.BaseRevisionID) == "" ||
		strings.TrimSpace(input.TargetRevisionID) == "" {
		return CompareResult{}, ErrInvalidRevisionService
	}
	base, err := service.dependencies.Repository.Get(
		ctx, input.AccountID, input.ContentID, input.BaseRevisionID,
	)
	if err != nil {
		return CompareResult{}, err
	}
	target, err := service.dependencies.Repository.Get(
		ctx, input.AccountID, input.ContentID, input.TargetRevisionID,
	)
	if err != nil {
		return CompareResult{}, err
	}
	if base.Revision.TranslationID != target.Revision.TranslationID {
		return CompareResult{}, errors.New("revisions belong to different translations")
	}
	baseSnapshot, err := decodeSnapshot(base.Snapshot, base.Revision.SnapshotHash)
	if err != nil {
		return CompareResult{}, err
	}
	targetSnapshot, err := decodeSnapshot(target.Snapshot, target.Revision.SnapshotHash)
	if err != nil {
		return CompareResult{}, err
	}
	changes := make([]FieldChange, 0)
	compareField := func(name string, from, to any) {
		if !reflect.DeepEqual(from, to) {
			changes = append(changes, FieldChange{Field: name, From: from, To: to})
		}
	}
	compareField("kind", baseSnapshot.Kind, targetSnapshot.Kind)
	compareField("content_key", baseSnapshot.ContentKey, targetSnapshot.ContentKey)
	compareField("slug", baseSnapshot.Slug, targetSnapshot.Slug)
	compareField("path", baseSnapshot.Path, targetSnapshot.Path)
	compareField("title", baseSnapshot.Title, targetSnapshot.Title)
	compareField("excerpt", baseSnapshot.Excerpt, targetSnapshot.Excerpt)
	compareField("blocks", baseSnapshot.Blocks, targetSnapshot.Blocks)
	compareField("seo", baseSnapshot.SEO, targetSnapshot.SEO)
	compareField("geo", baseSnapshot.GEO, targetSnapshot.GEO)
	compareField("aeo", baseSnapshot.AEO, targetSnapshot.AEO)
	compareField("status", baseSnapshot.Status, targetSnapshot.Status)
	return CompareResult{Base: base, Target: target, Changes: changes}, nil
}
