package app

import (
	"context"
	"errors"
	"strings"
	"time"

	publishinggorm "github.com/korwlik23/go-api-starter/internal/modules/publishing/adapters/gorm"
	publishingapplication "github.com/korwlik23/go-api-starter/internal/modules/publishing/application"
	schedulesapp "github.com/korwlik23/go-api-starter/internal/modules/publishing/application/schedules"
	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
	gormdb "gorm.io/gorm"
)

var errInvalidScheduleExecutor = errors.New("app: invalid schedule executor")

type schedulePublicationExecutor struct {
	db           *gormdb.DB
	workflow     *publishinggorm.WorkflowRepository
	transactions *database.TransactionManager
	invalidator  publishingapplication.CacheInvalidator
	now          func() time.Time
	newID        func() (string, error)
}

func newSchedulePublicationExecutor(
	db *gormdb.DB,
	workflow *publishinggorm.WorkflowRepository,
	transactions *database.TransactionManager,
	invalidator publishingapplication.CacheInvalidator,
) *schedulePublicationExecutor {
	return &schedulePublicationExecutor{
		db: db, workflow: workflow, transactions: transactions,
		invalidator: invalidator, now: time.Now, newID: newRuntimeID,
	}
}

type scheduledTranslationRecord struct {
	ContentID databaseid.ID `gorm:"column:content_id"`
	LocaleID  databaseid.ID `gorm:"column:locale_id"`
	Status    string        `gorm:"column:status"`
	Version   int64         `gorm:"column:version"`
}

func (executor *schedulePublicationExecutor) Execute(
	ctx context.Context,
	schedule publishingdomain.PublicationSchedule,
) error {
	if executor == nil || executor.db == nil || executor.workflow == nil ||
		executor.transactions == nil || executor.now == nil || executor.newID == nil ||
		ctx == nil || strings.TrimSpace(schedule.AccountID) == "" ||
		strings.TrimSpace(schedule.TranslationID) == "" || schedule.ExpectedVersion < 1 {
		return errInvalidScheduleExecutor
	}
	var detail ports.ContentDetail
	err := executor.transactions.WithinTransaction(ctx, func(transactionContext context.Context) error {
		connection, err := executor.transactions.Connection(transactionContext)
		if err != nil {
			return err
		}
		accountID, err := databaseid.Encode(executor.db.Dialector.Name(), schedule.AccountID)
		if err != nil {
			return err
		}
		translationID, err := databaseid.Encode(executor.db.Dialector.Name(), schedule.TranslationID)
		if err != nil {
			return err
		}
		var record scheduledTranslationRecord
		result := connection.Table("content_translations").Select("content_id, locale_id, status, version").Where(
			"account_id = ? AND id = ? AND deleted_at IS NULL", accountID, translationID,
		).Limit(1).Scan(&record)
		if result.Error != nil || result.RowsAffected != 1 || record.ContentID.String() == "" || record.LocaleID.String() == "" {
			return ports.ErrScheduleNotFound
		}
		if publishingdomain.Status(record.Status) != publishingdomain.StatusScheduled || record.Version != schedule.ExpectedVersion {
			return ports.ErrScheduleConflict
		}
		now := executor.now().UTC()
		if now.IsZero() {
			return errInvalidScheduleExecutor
		}
		eventID, err := executor.newID()
		if err != nil {
			return err
		}
		detail, err = executor.workflow.Transition(transactionContext, ports.WorkflowTransitionCommand{
			AccountID: schedule.AccountID, ContentID: record.ContentID.String(),
			TranslationID: schedule.TranslationID, LocaleID: record.LocaleID.String(),
			ExpectedVersion: record.Version, FromStatus: publishingdomain.StatusScheduled,
			ToStatus: publishingdomain.StatusPublished, UpdatedAt: now,
		})
		if err != nil {
			return err
		}
		return executor.workflow.AppendEvent(transactionContext, ports.WorkflowEvent{
			ID: eventID, AccountID: schedule.AccountID, ContentID: record.ContentID.String(),
			TranslationID: schedule.TranslationID, ActorID: eventID,
			FromStatus: publishingdomain.StatusScheduled, ToStatus: publishingdomain.StatusPublished,
			RequestID: "publishing.schedule." + schedule.ID, OccurredAt: now,
		})
	})
	if err != nil {
		return err
	}
	if executor.invalidator != nil {
		_ = executor.invalidator.InvalidateAfterCommit(ctx, publishingapplication.CacheInvalidationEvent{
			Namespace: "publishing.content", AccountID: schedule.AccountID,
			ContentID: detail.ContentID, LocaleID: detail.LocaleID,
		})
	}
	return nil
}

var _ schedulesapp.PublicationExecutor = (*schedulePublicationExecutor)(nil)
