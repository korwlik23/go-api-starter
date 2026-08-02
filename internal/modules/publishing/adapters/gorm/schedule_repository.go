package gorm

import (
	"context"
	"fmt"
	"time"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
	gormdb "gorm.io/gorm"
)

type ScheduleRepository struct {
	db          *gormdb.DB
	connections database.ConnectionProvider
	dialect     string
}

type scheduleRecord struct {
	ID              databaseid.ID `gorm:"column:id"`
	AccountID       databaseid.ID `gorm:"column:account_id"`
	TranslationID   databaseid.ID `gorm:"column:translation_id"`
	PublishAt       time.Time     `gorm:"column:publish_at"`
	UnpublishAt     *time.Time    `gorm:"column:unpublish_at"`
	ExpectedVersion int64         `gorm:"column:expected_version"`
	IdempotencyKey  string        `gorm:"column:idempotency_key"`
	State           string        `gorm:"column:state"`
	LeaseUntil      *time.Time    `gorm:"column:lease_until"`
	Attempts        int           `gorm:"column:attempts"`
	FailureReason   string        `gorm:"column:failure_reason"`
}

func NewScheduleRepository(db *gormdb.DB) (*ScheduleRepository, error) {
	if db == nil {
		return nil, ErrInvalidContentRepository
	}
	manager, err := database.NewTransactionManager(db)
	if err != nil {
		return nil, ErrInvalidContentRepository
	}
	return NewScheduleRepositoryWithDependencies(db, manager)
}

func NewScheduleRepositoryWithDependencies(
	db *gormdb.DB,
	connections database.ConnectionProvider,
) (*ScheduleRepository, error) {
	if db == nil || connections == nil {
		return nil, ErrInvalidContentRepository
	}
	return &ScheduleRepository{db: db, connections: connections, dialect: db.Dialector.Name()}, nil
}

func (repository *ScheduleRepository) connection(ctx context.Context) (*gormdb.DB, error) {
	if repository == nil || repository.connections == nil || ctx == nil {
		return nil, ports.ErrInvalidScheduleRepository
	}
	return repository.connections.Connection(ctx)
}

func (repository *ScheduleRepository) Create(ctx context.Context, command ports.ScheduleCreateCommand) error {
	db, err := repository.connection(ctx)
	if err != nil {
		return err
	}
	schedule := command.Schedule
	id, err := databaseid.Encode(repository.dialect, schedule.ID)
	if err != nil {
		return ports.ErrInvalidScheduleRepository
	}
	accountID, err := databaseid.Encode(repository.dialect, schedule.AccountID)
	if err != nil {
		return ports.ErrInvalidScheduleRepository
	}
	translationID, err := databaseid.Encode(repository.dialect, schedule.TranslationID)
	if err != nil {
		return ports.ErrInvalidScheduleRepository
	}
	if err := db.Table("publication_schedules").Create(map[string]any{
		"id": id, "account_id": accountID, "translation_id": translationID,
		"publish_at": schedule.PublishAt, "unpublish_at": schedule.UnpublishAt,
		"expected_version": schedule.ExpectedVersion, "idempotency_key": schedule.IdempotencyKey,
		"state": schedule.State, "lease_until": schedule.LeaseUntil, "attempts": schedule.Attempts,
		"failure_reason": schedule.FailureReason, "created_at": time.Now().UTC(), "updated_at": time.Now().UTC(),
	}).Error; err != nil {
		return fmt.Errorf("create publication schedule: %w", err)
	}
	return nil
}

func (repository *ScheduleRepository) FindByIdempotency(ctx context.Context, accountID, key string) (publishingdomain.PublicationSchedule, error) {
	db, err := repository.connection(ctx)
	if err != nil {
		return publishingdomain.PublicationSchedule{}, err
	}
	encoded, err := databaseid.Encode(repository.dialect, accountID)
	if err != nil {
		return publishingdomain.PublicationSchedule{}, ports.ErrScheduleNotFound
	}
	var record scheduleRecord
	result := db.WithContext(ctx).Table("publication_schedules").Select(
		"id, account_id, translation_id, publish_at, unpublish_at, expected_version, idempotency_key, state, lease_until, attempts, failure_reason",
	).Where("account_id = ? AND idempotency_key = ?", encoded, key).Limit(1).Scan(&record)
	if result.Error != nil {
		return publishingdomain.PublicationSchedule{}, fmt.Errorf("find publication schedule: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return publishingdomain.PublicationSchedule{}, ports.ErrScheduleNotFound
	}
	return mapScheduleRecord(record)
}

func (repository *ScheduleRepository) List(ctx context.Context, query ports.ScheduleListQuery) (ports.SchedulePage, error) {
	if query.Limit < 1 || query.Limit > 100 {
		return ports.SchedulePage{}, ports.ErrInvalidScheduleRepository
	}
	db, err := repository.connection(ctx)
	if err != nil {
		return ports.SchedulePage{}, err
	}
	accountID, err := databaseid.Encode(repository.dialect, query.AccountID)
	if err != nil {
		return ports.SchedulePage{}, ports.ErrInvalidScheduleRepository
	}
	var records []scheduleRecord
	if err := db.WithContext(ctx).Table("publication_schedules").Select(
		"id, account_id, translation_id, publish_at, unpublish_at, expected_version, idempotency_key, state, lease_until, attempts, failure_reason",
	).Where("account_id = ?", accountID).Order("publish_at DESC, id DESC").Limit(query.Limit).Find(&records).Error; err != nil {
		return ports.SchedulePage{}, fmt.Errorf("list publication schedules: %w", err)
	}
	page := ports.SchedulePage{Items: make([]publishingdomain.PublicationSchedule, 0, len(records))}
	for _, record := range records {
		schedule, mapErr := mapScheduleRecord(record)
		if mapErr != nil {
			return ports.SchedulePage{}, mapErr
		}
		page.Items = append(page.Items, schedule)
	}
	return page, nil
}

func (repository *ScheduleRepository) Cancel(ctx context.Context, command ports.ScheduleCancelCommand) error {
	db, err := repository.connection(ctx)
	if err != nil {
		return err
	}
	accountID, err := databaseid.Encode(repository.dialect, command.AccountID)
	if err != nil {
		return ports.ErrScheduleNotFound
	}
	scheduleID, err := databaseid.Encode(repository.dialect, command.ScheduleID)
	if err != nil {
		return ports.ErrScheduleNotFound
	}
	result := db.Table("publication_schedules").Where(
		"account_id = ? AND id = ? AND state IN ?", accountID, scheduleID,
		[]publishingdomain.ScheduleState{publishingdomain.SchedulePending, publishingdomain.ScheduleLeased},
	).Updates(map[string]any{"state": publishingdomain.ScheduleCanceled, "lease_until": nil, "updated_at": command.CanceledAt.UTC()})
	if result.Error != nil {
		return fmt.Errorf("cancel publication schedule: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ports.ErrScheduleConflict
	}
	return nil
}

func (repository *ScheduleRepository) ClaimDue(ctx context.Context, now, leaseUntil time.Time, limit int) ([]publishingdomain.PublicationSchedule, error) {
	if limit < 1 || limit > 100 || now.IsZero() || leaseUntil.IsZero() {
		return nil, ports.ErrInvalidScheduleRepository
	}
	db, err := repository.connection(ctx)
	if err != nil {
		return nil, err
	}
	var records []scheduleRecord
	if err := db.WithContext(ctx).Table("publication_schedules").Select(
		"id, account_id, translation_id, publish_at, unpublish_at, expected_version, idempotency_key, state, lease_until, attempts, failure_reason",
	).Where("state = ? AND publish_at <= ? AND (lease_until IS NULL OR lease_until < ?)", publishingdomain.SchedulePending, now, now).
		Order("publish_at ASC, id ASC").Limit(limit).Find(&records).Error; err != nil {
		return nil, fmt.Errorf("claim due schedules: %w", err)
	}
	claimed := make([]publishingdomain.PublicationSchedule, 0, len(records))
	for _, record := range records {
		id, encodeErr := databaseid.Encode(repository.dialect, record.ID.String())
		if encodeErr != nil {
			return nil, ports.ErrInvalidScheduleRepository
		}
		result := db.Table("publication_schedules").Where(
			"id = ? AND state = ? AND publish_at <= ? AND (lease_until IS NULL OR lease_until < ?)", id, publishingdomain.SchedulePending, now, now,
		).Updates(map[string]any{"state": publishingdomain.ScheduleLeased, "lease_until": leaseUntil.UTC(), "attempts": gormdb.Expr("attempts + ?", 1), "updated_at": now.UTC()})
		if result.Error != nil {
			return nil, fmt.Errorf("lease publication schedule: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			continue
		}
		record.State = string(publishingdomain.ScheduleLeased)
		record.LeaseUntil = &leaseUntil
		record.Attempts++
		schedule, mapErr := mapScheduleRecord(record)
		if mapErr != nil {
			return nil, mapErr
		}
		claimed = append(claimed, schedule)
	}
	return claimed, nil
}

func (repository *ScheduleRepository) Finish(ctx context.Context, command ports.ScheduleFinishCommand) error {
	if err := ports.ValidateScheduleFinish(command); err != nil {
		return err
	}
	db, err := repository.connection(ctx)
	if err != nil {
		return err
	}
	accountID, err := databaseid.Encode(repository.dialect, command.AccountID)
	if err != nil {
		return ports.ErrScheduleNotFound
	}
	scheduleID, err := databaseid.Encode(repository.dialect, command.ScheduleID)
	if err != nil {
		return ports.ErrScheduleNotFound
	}
	values := map[string]any{"state": command.State, "lease_until": nil, "failure_reason": command.FailureReason, "updated_at": command.FinishedAt.UTC()}
	if command.NextAttemptAt != nil {
		values["publish_at"] = command.NextAttemptAt.UTC()
	}
	result := db.Table("publication_schedules").Where("account_id = ? AND id = ? AND state = ? AND attempts = ?", accountID, scheduleID, publishingdomain.ScheduleLeased, command.ExpectedAttempts).Updates(values)
	if result.Error != nil {
		return fmt.Errorf("finish publication schedule: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ports.ErrScheduleConflict
	}
	return nil
}

func mapScheduleRecord(record scheduleRecord) (publishingdomain.PublicationSchedule, error) {
	schedule, err := publishingdomain.NewPublicationSchedule(publishingdomain.PublicationScheduleInput{
		ID: record.ID.String(), AccountID: record.AccountID.String(), TranslationID: record.TranslationID.String(),
		PublishAt: record.PublishAt, UnpublishAt: record.UnpublishAt, ExpectedVersion: record.ExpectedVersion,
		IdempotencyKey: record.IdempotencyKey, State: publishingdomain.ScheduleState(record.State),
		LeaseUntil: record.LeaseUntil, Attempts: record.Attempts, FailureReason: record.FailureReason,
	})
	if err != nil {
		return publishingdomain.PublicationSchedule{}, ports.ErrInvalidScheduleRepository
	}
	return schedule, nil
}

var _ ports.ScheduleRepository = (*ScheduleRepository)(nil)
