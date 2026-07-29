package ports

import "context"

type PermissionRecord struct {
	ID        string
	Key       string
	ModuleID  string
	Scope     string
	Tier      string
	Delegable bool
	Active    bool
}

type ModuleRecord struct {
	ID                string
	Enabled           bool
	ModuleChecksum    []byte
	ConfigChecksum    []byte
	CatalogChecksum   []byte
	CatalogEpoch      int64
	ReconcileRevision int64
}

type ReconcileSnapshot struct {
	Revision     int64
	CatalogEpoch int64
	Modules      []ModuleRecord
	Permissions  []PermissionRecord
}

type ReconcileProjection struct {
	Revision     int64
	CatalogEpoch int64
	Modules      []ModuleRecord
	Permissions  []PermissionRecord
}

type ReconcileSession interface {
	Snapshot(context.Context) (ReconcileSnapshot, error)
	Apply(context.Context, ReconcileProjection) error
}

type ModuleStateRepository interface {
	WithinReconcile(
		context.Context,
		func(context.Context, ReconcileSession) error,
	) error
}
