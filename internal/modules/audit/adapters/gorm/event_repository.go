package gorm

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	auditdomain "github.com/korwlik23/go-api-starter/internal/modules/audit/domain"
	auditports "github.com/korwlik23/go-api-starter/internal/modules/audit/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
	gormdb "gorm.io/gorm"
)

var (
	ErrInvalidEventRepository = errors.New("audit: invalid event repository")
	ErrEventRepositoryFailed  = errors.New("audit: event repository failed")
)

const maximumAuditPageSize = 200

type EventRepository struct {
	connections database.ConnectionProvider
	dialect     string
}

type eventRecord struct {
	ID          databaseid.ID  `gorm:"column:id"`
	Scope       string         `gorm:"column:scope"`
	AccountID   *databaseid.ID `gorm:"column:account_id"`
	ActorType   string         `gorm:"column:actor_type"`
	ActorID     *databaseid.ID `gorm:"column:actor_id"`
	Action      string         `gorm:"column:action"`
	OperationID databaseid.ID  `gorm:"column:operation_id"`
	RequestID   *string        `gorm:"column:request_id"`
	EventData   string         `gorm:"column:event_data"`
	OccurredAt  time.Time      `gorm:"column:occurred_at"`
	RecordedAt  time.Time      `gorm:"column:recorded_at"`
}

func NewEventRepository(db *gormdb.DB) (*EventRepository, error) {
	if db == nil {
		return nil, ErrInvalidEventRepository
	}
	return &EventRepository{
		connections: staticConnectionProvider{db: db},
		dialect:     db.Dialector.Name(),
	}, nil
}

func NewTransactionalEventRepository(
	connections database.ConnectionProvider,
	dialect string,
) (*EventRepository, error) {
	if connections == nil || (dialect != "postgres" && dialect != "mysql") {
		return nil, ErrInvalidEventRepository
	}
	return &EventRepository{
		connections: connections,
		dialect:     dialect,
	}, nil
}

func (repository *EventRepository) Append(
	ctx context.Context,
	event auditdomain.Event,
) error {
	if repository == nil ||
		repository.connections == nil ||
		ctx == nil ||
		!event.RecordedAt.IsZero() {
		return ErrInvalidEventRepository
	}
	db, err := repository.connections.Connection(ctx)
	if err != nil {
		return ErrEventRepositoryFailed
	}
	id, err := databaseid.Encode(repository.dialect, event.ID)
	if err != nil {
		return ErrInvalidEventRepository
	}
	operationID, err := databaseid.Encode(
		repository.dialect,
		event.OperationID,
	)
	if err != nil {
		return ErrInvalidEventRepository
	}
	accountID, err := optionalDatabaseID(
		repository.dialect,
		event.AccountID,
	)
	if err != nil {
		return ErrInvalidEventRepository
	}
	actorID, err := optionalDatabaseID(
		repository.dialect,
		event.Actor.ID,
	)
	if err != nil {
		return ErrInvalidEventRepository
	}
	values := map[string]any{
		"id":           id,
		"scope":        string(event.Scope),
		"account_id":   accountID,
		"actor_type":   string(event.Actor.Type),
		"actor_id":     actorID,
		"action":       event.Action,
		"operation_id": operationID,
		"request_id":   optionalString(event.RequestID),
		"event_data":   string(event.Data),
		"occurred_at":  event.OccurredAt,
	}
	if err := db.WithContext(ctx).
		Table("audit_events").
		Create(values).Error; err != nil {
		return ErrEventRepositoryFailed
	}
	return nil
}

func (repository *EventRepository) ListAccount(
	ctx context.Context,
	accountID string,
	cursor *auditports.Cursor,
	limit int,
) (auditports.EventPage, error) {
	if repository == nil || repository.connections == nil || ctx == nil {
		return auditports.EventPage{}, ErrInvalidEventRepository
	}
	db, connectionErr := repository.connections.Connection(ctx)
	if connectionErr != nil {
		return auditports.EventPage{}, ErrEventRepositoryFailed
	}
	encodedAccountID, err := databaseid.Encode(
		repository.dialect,
		accountID,
	)
	if err != nil {
		return auditports.EventPage{}, ErrInvalidEventRepository
	}
	query := db.WithContext(ctx).
		Table("audit_events").
		Where("scope = ? AND account_id = ?", string(auditdomain.ScopeAccount), encodedAccountID)
	return repository.list(ctx, query, cursor, limit)
}

func (repository *EventRepository) ListSystem(
	ctx context.Context,
	cursor *auditports.Cursor,
	limit int,
) (auditports.EventPage, error) {
	if repository == nil || repository.connections == nil || ctx == nil {
		return auditports.EventPage{}, ErrInvalidEventRepository
	}
	db, err := repository.connections.Connection(ctx)
	if err != nil {
		return auditports.EventPage{}, ErrEventRepositoryFailed
	}
	query := db.WithContext(ctx).
		Table("audit_events").
		Where("scope = ? AND account_id IS NULL", string(auditdomain.ScopeSystem))
	return repository.list(ctx, query, cursor, limit)
}

func (repository *EventRepository) ListSystemFiltered(
	ctx context.Context,
	filter auditports.SystemEventFilter,
) (auditports.EventPage, error) {
	if repository == nil || repository.connections == nil || ctx == nil {
		return auditports.EventPage{}, ErrInvalidEventRepository
	}
	db, err := repository.connections.Connection(ctx)
	if err != nil {
		return auditports.EventPage{}, ErrEventRepositoryFailed
	}
	query := db.WithContext(ctx).Table("audit_events").
		Where("scope = ? AND account_id IS NULL", string(auditdomain.ScopeSystem))
	if filter.Action != "" {
		query = query.Where("action = ?", filter.Action)
	}
	if filter.ActorID != "" {
		encoded, encodeErr := databaseid.Encode(repository.dialect, filter.ActorID)
		if encodeErr != nil {
			return auditports.EventPage{}, ErrInvalidEventRepository
		}
		query = query.Where("actor_type = ? AND actor_id = ?", string(auditdomain.ActorUser), encoded)
	}
	return repository.list(ctx, query, filter.Cursor, filter.Limit)
}

func (repository *EventRepository) list(
	ctx context.Context,
	query *gormdb.DB,
	cursor *auditports.Cursor,
	limit int,
) (auditports.EventPage, error) {
	if repository == nil ||
		repository.connections == nil ||
		ctx == nil ||
		query == nil ||
		limit < 1 ||
		limit > maximumAuditPageSize {
		return auditports.EventPage{}, ErrInvalidEventRepository
	}
	if cursor != nil {
		if cursor.RecordedAt.IsZero() || cursor.ID == "" {
			return auditports.EventPage{}, ErrInvalidEventRepository
		}
		encodedID, err := databaseid.Encode(repository.dialect, cursor.ID)
		if err != nil {
			return auditports.EventPage{}, ErrInvalidEventRepository
		}
		query = query.Where(
			"(recorded_at < ?) OR (recorded_at = ? AND id < ?)",
			cursor.RecordedAt,
			cursor.RecordedAt,
			encodedID,
		)
	}
	var records []eventRecord
	if err := query.
		Order("recorded_at DESC").
		Order("id DESC").
		Limit(limit + 1).
		Find(&records).Error; err != nil {
		return auditports.EventPage{}, ErrEventRepositoryFailed
	}
	hasMore := len(records) > limit
	if hasMore {
		records = records[:limit]
	}
	items := make([]auditdomain.Event, 0, len(records))
	for _, record := range records {
		event, err := eventFromRecord(record)
		if err != nil {
			return auditports.EventPage{}, err
		}
		items = append(items, event)
	}
	page := auditports.EventPage{Items: items}
	if hasMore && len(records) > 0 {
		last := records[len(records)-1]
		page.NextCursor = &auditports.Cursor{
			RecordedAt: last.RecordedAt,
			ID:         last.ID.String(),
		}
	}
	return page, nil
}

func eventFromRecord(record eventRecord) (auditdomain.Event, error) {
	data := make(map[string]any)
	if err := json.Unmarshal([]byte(record.EventData), &data); err != nil {
		return auditdomain.Event{}, ErrEventRepositoryFailed
	}
	event, err := auditdomain.NewEvent(auditdomain.EventInput{
		ID:        record.ID.String(),
		Scope:     auditdomain.Scope(record.Scope),
		AccountID: databaseIDString(record.AccountID),
		Actor: auditdomain.Actor{
			Type: auditdomain.ActorType(record.ActorType),
			ID:   databaseIDString(record.ActorID),
		},
		Action:      record.Action,
		OperationID: record.OperationID.String(),
		RequestID:   stringValue(record.RequestID),
		Data:        data,
		OccurredAt:  record.OccurredAt,
	})
	if err != nil || record.RecordedAt.IsZero() {
		return auditdomain.Event{}, ErrEventRepositoryFailed
	}
	event.RecordedAt = record.RecordedAt
	return event, nil
}

func optionalDatabaseID(dialect, value string) (any, error) {
	if value == "" {
		return nil, nil
	}
	return databaseid.Encode(dialect, value)
}

func optionalString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func databaseIDString(value *databaseid.ID) string {
	if value == nil {
		return ""
	}
	return value.String()
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

var _ auditports.EventRepository = (*EventRepository)(nil)
var _ auditports.SystemEventQuery = (*EventRepository)(nil)

type staticConnectionProvider struct {
	db *gormdb.DB
}

func (provider staticConnectionProvider) Connection(
	ctx context.Context,
) (*gormdb.DB, error) {
	if provider.db == nil || ctx == nil {
		return nil, ErrInvalidEventRepository
	}
	return provider.db.WithContext(ctx), nil
}
