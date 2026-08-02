package gorm

import (
	"context"
	"errors"
	"fmt"
	"time"

	mediadomain "github.com/korwlik23/go-api-starter/internal/modules/media/domain"
	mediaports "github.com/korwlik23/go-api-starter/internal/modules/media/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
	gormdb "gorm.io/gorm"
)

var ErrInvalidAssetRepository = errors.New("invalid media asset repository")

type AssetRepository struct {
	connections  database.ConnectionProvider
	transactions database.Transactor
	dialect      string
}

type assetRecord struct {
	ID           databaseid.ID `gorm:"column:id"`
	AccountID    databaseid.ID `gorm:"column:account_id"`
	StorageKey   string        `gorm:"column:storage_key"`
	OriginalName string        `gorm:"column:original_name"`
	MIMEType     string        `gorm:"column:mime_type"`
	ByteSize     int64         `gorm:"column:byte_size"`
	SHA256       string        `gorm:"column:sha256"`
	Width        *int64        `gorm:"column:width"`
	Height       *int64        `gorm:"column:height"`
	Version      int64         `gorm:"column:version"`
	DeletedAt    *time.Time    `gorm:"column:deleted_at"`
	CreatedAt    time.Time     `gorm:"column:created_at"`
	UpdatedAt    time.Time     `gorm:"column:updated_at"`
}

type uploadRecord struct {
	ID             databaseid.ID  `gorm:"column:id"`
	AccountID      databaseid.ID  `gorm:"column:account_id"`
	StorageKey     string         `gorm:"column:storage_key"`
	OriginalName   string         `gorm:"column:original_name"`
	MIMEType       string         `gorm:"column:mime_type"`
	DeclaredSize   int64          `gorm:"column:declared_size"`
	ExpectedSHA256 string         `gorm:"column:expected_sha256"`
	State          string         `gorm:"column:state"`
	AssetID        *databaseid.ID `gorm:"column:asset_id"`
	Version        int64          `gorm:"column:version"`
	ExpiresAt      time.Time      `gorm:"column:expires_at"`
	CreatedAt      time.Time      `gorm:"column:created_at"`
	UpdatedAt      time.Time      `gorm:"column:updated_at"`
}

func NewAssetRepository(db *gormdb.DB) (*AssetRepository, error) {
	if db == nil || db.Dialector == nil {
		return nil, ErrInvalidAssetRepository
	}
	transactions, err := database.NewTransactionManager(db)
	if err != nil {
		return nil, ErrInvalidAssetRepository
	}
	return &AssetRepository{connections: transactions, transactions: transactions, dialect: db.Dialector.Name()}, nil
}

func NewTransactionalAssetRepository(connections database.ConnectionProvider, dialect string) (*AssetRepository, error) {
	if connections == nil || (dialect != "postgres" && dialect != "mysql") {
		return nil, ErrInvalidAssetRepository
	}
	transactions, _ := connections.(database.Transactor)
	return &AssetRepository{connections: connections, transactions: transactions, dialect: dialect}, nil
}

func (repository *AssetRepository) connection(ctx context.Context) (*gormdb.DB, error) {
	if repository == nil || repository.connections == nil || ctx == nil {
		return nil, ErrInvalidAssetRepository
	}
	return repository.connections.Connection(ctx)
}

func (repository *AssetRepository) CreateUpload(ctx context.Context, upload mediadomain.Upload) error {
	if repository == nil || upload.Version != 1 || upload.State != mediadomain.UploadPending {
		return ErrInvalidAssetRepository
	}
	db, err := repository.connection(ctx)
	if err != nil {
		return err
	}
	values, err := repository.encodeUploadIDs(upload)
	if err != nil {
		return ErrInvalidAssetRepository
	}
	if err := db.Table("media_uploads").Create(map[string]any{
		"id": values.id, "account_id": values.accountID, "storage_key": upload.StorageKey,
		"original_name": upload.OriginalName, "mime_type": upload.MIMEType, "declared_size": upload.DeclaredSize,
		"expected_sha256": upload.ExpectedSHA256, "state": string(upload.State), "version": upload.Version,
		"expires_at": upload.ExpiresAt, "created_at": upload.CreatedAt, "updated_at": upload.UpdatedAt,
	}).Error; err != nil {
		return fmt.Errorf("create media upload: %w", err)
	}
	return nil
}

func (repository *AssetRepository) GetUpload(ctx context.Context, accountID, uploadID string) (mediadomain.Upload, error) {
	db, err := repository.connection(ctx)
	if err != nil {
		return mediadomain.Upload{}, err
	}
	encodedAccount, encodedUpload, err := repository.encodeIDs(accountID, uploadID)
	if err != nil {
		return mediadomain.Upload{}, mediaports.ErrUploadNotFound
	}
	var record uploadRecord
	result := db.Table("media_uploads").Select("id, account_id, storage_key, original_name, mime_type, declared_size, expected_sha256, state, asset_id, version, expires_at, created_at, updated_at").Where("account_id = ? AND id = ?", encodedAccount, encodedUpload).Limit(1).Scan(&record)
	if result.Error != nil {
		return mediadomain.Upload{}, fmt.Errorf("get media upload: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return mediadomain.Upload{}, mediaports.ErrUploadNotFound
	}
	return mapUploadRecord(record)
}

func (repository *AssetRepository) FailUpload(ctx context.Context, command mediaports.FailUploadCommand) error {
	if command.ExpectedVersion < 1 || (command.State != mediadomain.UploadFailed && command.State != mediadomain.UploadExpired) || command.UpdatedAt.IsZero() {
		return ErrInvalidAssetRepository
	}
	db, err := repository.connection(ctx)
	if err != nil {
		return err
	}
	accountID, uploadID, err := repository.encodeIDs(command.AccountID, command.UploadID)
	if err != nil {
		return mediaports.ErrUploadNotFound
	}
	result := db.Table("media_uploads").Where("account_id = ? AND id = ? AND version = ? AND state = ?", accountID, uploadID, command.ExpectedVersion, string(mediadomain.UploadPending)).Updates(map[string]any{"state": string(command.State), "version": command.ExpectedVersion + 1, "updated_at": command.UpdatedAt})
	if result.Error != nil {
		return fmt.Errorf("fail media upload: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return mediaports.ErrUploadConflict
	}
	return nil
}

func (repository *AssetRepository) FinalizeUpload(ctx context.Context, command mediaports.FinalizeAssetCommand) (result mediadomain.Asset, resultErr error) {
	if command.ExpectedVersion < 1 || command.FinalizedAt.IsZero() || command.Asset.Version != 1 || command.Asset.AccountID != command.AccountID {
		return mediadomain.Asset{}, ErrInvalidAssetRepository
	}
	if repository == nil || repository.transactions == nil {
		return mediadomain.Asset{}, ErrInvalidAssetRepository
	}
	resultErr = repository.transactions.WithinTransaction(ctx, func(transactionContext context.Context) error {
		db, err := repository.connection(transactionContext)
		if err != nil {
			return err
		}
		assetValues, err := repository.encodeAssetIDs(command.Asset)
		if err != nil {
			return ErrInvalidAssetRepository
		}
		if err := db.Table("media_assets").Create(map[string]any{
			"id": assetValues.id, "account_id": assetValues.accountID, "storage_key": command.Asset.StorageKey,
			"original_name": command.Asset.OriginalName, "mime_type": command.Asset.MIMEType, "byte_size": command.Asset.ByteSize,
			"sha256": command.Asset.SHA256, "width": nullableDimension(command.Asset.Width), "height": nullableDimension(command.Asset.Height),
			"version": command.Asset.Version, "created_at": command.Asset.CreatedAt, "updated_at": command.Asset.UpdatedAt,
		}).Error; err != nil {
			return fmt.Errorf("create media asset: %w", err)
		}
		accountID, uploadID, encodeErr := repository.encodeIDs(command.AccountID, command.UploadID)
		if encodeErr != nil {
			return mediaports.ErrUploadNotFound
		}
		assetID, encodeErr := databaseid.Encode(repository.dialect, command.Asset.ID)
		if encodeErr != nil {
			return ErrInvalidAssetRepository
		}
		update := db.Table("media_uploads").Where("account_id = ? AND id = ? AND version = ? AND state = ?", accountID, uploadID, command.ExpectedVersion, string(mediadomain.UploadPending)).Updates(map[string]any{"state": string(mediadomain.UploadFinalized), "asset_id": assetID, "version": command.ExpectedVersion + 1, "updated_at": command.FinalizedAt})
		if update.Error != nil {
			return fmt.Errorf("finalize media upload: %w", update.Error)
		}
		if update.RowsAffected != 1 {
			return mediaports.ErrUploadConflict
		}
		result = command.Asset
		return nil
	})
	return result, resultErr
}

func (repository *AssetRepository) ListAssets(ctx context.Context, query mediaports.AssetListQuery) (mediaports.AssetListPage, error) {
	if err := mediaports.ValidateAssetListQuery(query); err != nil {
		return mediaports.AssetListPage{}, err
	}
	db, err := repository.connection(ctx)
	if err != nil {
		return mediaports.AssetListPage{}, err
	}
	accountID, err := databaseid.Encode(repository.dialect, query.AccountID)
	if err != nil {
		return mediaports.AssetListPage{}, mediaports.ErrInvalidAssetQuery
	}
	db = db.Table("media_assets").Select("id, account_id, storage_key, original_name, mime_type, byte_size, sha256, width, height, version, deleted_at, created_at, updated_at").Where("account_id = ? AND deleted_at IS NULL", accountID)
	if query.After != nil {
		cursorID, encodeErr := databaseid.Encode(repository.dialect, query.After.ID)
		if encodeErr != nil {
			return mediaports.AssetListPage{}, mediaports.ErrInvalidAssetQuery
		}
		db = db.Where("(updated_at < ? OR (updated_at = ? AND id < ?))", query.After.UpdatedAt, query.After.UpdatedAt, cursorID)
	}
	var records []assetRecord
	if err := db.Order("updated_at DESC, id DESC").Limit(query.Limit + 1).Scan(&records).Error; err != nil {
		return mediaports.AssetListPage{}, fmt.Errorf("list media assets: %w", err)
	}
	page := mediaports.AssetListPage{Items: make([]mediadomain.Asset, 0, query.Limit)}
	if len(records) > query.Limit {
		records = records[:query.Limit]
		last := records[len(records)-1]
		page.Next = &mediaports.AssetCursor{UpdatedAt: last.UpdatedAt.UTC(), ID: last.ID.String()}
	}
	for _, record := range records {
		asset, mapErr := mapAssetRecord(record)
		if mapErr != nil {
			return mediaports.AssetListPage{}, mapErr
		}
		page.Items = append(page.Items, asset)
	}
	return page, nil
}

func (repository *AssetRepository) SoftDeleteAsset(ctx context.Context, command mediaports.DeleteAssetCommand) (mediadomain.Asset, error) {
	if command.ExpectedVersion < 1 || command.DeletedAt.IsZero() {
		return mediadomain.Asset{}, ErrInvalidAssetRepository
	}
	db, err := repository.connection(ctx)
	if err != nil {
		return mediadomain.Asset{}, err
	}
	accountID, assetID, err := repository.encodeIDs(command.AccountID, command.AssetID)
	if err != nil {
		return mediadomain.Asset{}, mediaports.ErrAssetNotFound
	}
	result := db.Table("media_assets").Where("account_id = ? AND id = ? AND version = ? AND deleted_at IS NULL", accountID, assetID, command.ExpectedVersion).Updates(map[string]any{"deleted_at": command.DeletedAt, "version": command.ExpectedVersion + 1, "updated_at": command.DeletedAt})
	if result.Error != nil {
		return mediadomain.Asset{}, fmt.Errorf("delete media asset: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return mediadomain.Asset{}, mediaports.ErrAssetConflict
	}
	return repository.getAsset(ctx, command.AccountID, command.AssetID, true)
}

func (repository *AssetRepository) getAsset(ctx context.Context, accountID, assetID string, includeDeleted bool) (mediadomain.Asset, error) {
	db, err := repository.connection(ctx)
	if err != nil {
		return mediadomain.Asset{}, err
	}
	encodedAccount, encodedAsset, err := repository.encodeIDs(accountID, assetID)
	if err != nil {
		return mediadomain.Asset{}, mediaports.ErrAssetNotFound
	}
	var record assetRecord
	query := db.Table("media_assets").Select("id, account_id, storage_key, original_name, mime_type, byte_size, sha256, width, height, version, deleted_at, created_at, updated_at").Where("account_id = ? AND id = ?", encodedAccount, encodedAsset)
	if !includeDeleted {
		query = query.Where("deleted_at IS NULL")
	}
	result := query.Limit(1).Scan(&record)
	if result.Error != nil {
		return mediadomain.Asset{}, fmt.Errorf("get media asset: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return mediadomain.Asset{}, mediaports.ErrAssetNotFound
	}
	return mapAssetRecord(record)
}

func (repository *AssetRepository) encodeIDs(accountID, objectID string) (any, any, error) {
	account, err := databaseid.Encode(repository.dialect, accountID)
	if err != nil {
		return nil, nil, err
	}
	object, err := databaseid.Encode(repository.dialect, objectID)
	return account, object, err
}

func (repository *AssetRepository) encodeUploadIDs(upload mediadomain.Upload) (encodedIDs, error) {
	account, id, err := repository.encodeIDs(upload.AccountID, upload.ID)
	return encodedIDs{id: id, accountID: account}, err
}

func (repository *AssetRepository) encodeAssetIDs(asset mediadomain.Asset) (encodedIDs, error) {
	account, id, err := repository.encodeIDs(asset.AccountID, asset.ID)
	return encodedIDs{id: id, accountID: account}, err
}

type encodedIDs struct{ id, accountID any }

func mapAssetRecord(record assetRecord) (mediadomain.Asset, error) {
	asset, err := mediadomain.NewAsset(mediadomain.AssetInput{ID: record.ID.String(), AccountID: record.AccountID.String(), StorageKey: record.StorageKey, OriginalName: record.OriginalName, MIMEType: record.MIMEType, ByteSize: record.ByteSize, SHA256: record.SHA256, Width: dimensionValue(record.Width), Height: dimensionValue(record.Height), Version: record.Version, DeletedAt: record.DeletedAt, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt})
	if err != nil {
		return mediadomain.Asset{}, ErrInvalidAssetRepository
	}
	return asset, nil
}

func mapUploadRecord(record uploadRecord) (mediadomain.Upload, error) {
	assetID := ""
	if record.AssetID != nil {
		assetID = record.AssetID.String()
	}
	upload, err := mediadomain.NewUpload(mediadomain.UploadInput{ID: record.ID.String(), AccountID: record.AccountID.String(), StorageKey: record.StorageKey, OriginalName: record.OriginalName, MIMEType: record.MIMEType, DeclaredSize: record.DeclaredSize, ExpectedSHA256: record.ExpectedSHA256, State: mediadomain.UploadState(record.State), AssetID: assetID, Version: record.Version, ExpiresAt: record.ExpiresAt, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt})
	if err != nil {
		return mediadomain.Upload{}, ErrInvalidAssetRepository
	}
	return upload, nil
}

func nullableDimension(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}

func dimensionValue(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

var _ mediaports.AssetRepository = (*AssetRepository)(nil)
