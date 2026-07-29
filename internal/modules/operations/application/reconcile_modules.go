package application

import (
	"context"
	"errors"

	"github.com/korwlik23/go-api-starter/internal/modular"
	"github.com/korwlik23/go-api-starter/internal/modules/operations/ports"
)

var (
	ErrInvalidReconcile         = errors.New("operations: invalid module reconcile")
	ErrStaleReconcile           = errors.New("operations: stale module reconcile")
	ErrPermissionOwnerCollision = errors.New("operations: permission owner collision")
	ErrReconcileFailed          = errors.New("operations: module reconcile failed")
)

type ReconcileRequest struct {
	Registry         *modular.Registry
	Enabled          map[string]bool
	ExpectedRevision int64
}

type ReconcileResult struct {
	Revision     int64
	CatalogEpoch int64
}

type ReconcileDependencies struct {
	Repository ports.ModuleStateRepository
	NewID      func() (string, error)
}

type ReconcileModules struct {
	dependencies ReconcileDependencies
}

func NewReconcileModules(
	dependencies ReconcileDependencies,
) (*ReconcileModules, error) {
	if dependencies.Repository == nil || dependencies.NewID == nil {
		return nil, ErrInvalidReconcile
	}
	return &ReconcileModules{dependencies: dependencies}, nil
}

func (reconciler *ReconcileModules) Execute(
	ctx context.Context,
	request ReconcileRequest,
) (ReconcileResult, error) {
	if reconciler == nil ||
		reconciler.dependencies.Repository == nil ||
		ctx == nil ||
		request.Registry == nil ||
		request.ExpectedRevision < 0 {
		return ReconcileResult{}, ErrInvalidReconcile
	}
	if err := ctx.Err(); err != nil {
		return ReconcileResult{}, err
	}

	result := ReconcileResult{}
	err := reconciler.dependencies.Repository.WithinReconcile(
		ctx,
		func(transactionContext context.Context, session ports.ReconcileSession) error {
			if session == nil {
				return ErrReconcileFailed
			}
			snapshot, snapshotError := session.Snapshot(transactionContext)
			if snapshotError != nil {
				return snapshotError
			}
			if snapshot.Revision != request.ExpectedRevision {
				return ErrStaleReconcile
			}
			projection, projectionError := buildReconcileProjection(
				request,
				snapshot,
				reconciler.dependencies.NewID,
			)
			if projectionError != nil {
				return projectionError
			}
			if applyError := session.Apply(transactionContext, projection); applyError != nil {
				return applyError
			}
			result = ReconcileResult{
				Revision:     projection.Revision,
				CatalogEpoch: projection.CatalogEpoch,
			}
			return nil
		},
	)
	if err == nil {
		return result, nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return ReconcileResult{}, err
	}
	if errors.Is(err, ErrInvalidReconcile) ||
		errors.Is(err, ErrStaleReconcile) ||
		errors.Is(err, ErrPermissionOwnerCollision) {
		return ReconcileResult{}, err
	}
	return ReconcileResult{}, ErrReconcileFailed
}
