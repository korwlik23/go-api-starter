package gorm

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/korwlik23/go-api-starter/internal/modules/operations/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/operations/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
	gormdb "gorm.io/gorm"
)

var ErrInvalidJobRepository = errors.New("operations: invalid job repository")

type JobRepository struct{ connections database.ConnectionProvider }

type jobRecord struct {
	ID        databaseid.ID `gorm:"column:id"`
	Name      string        `gorm:"column:name"`
	Payload   []byte        `gorm:"column:payload"`
	State     string        `gorm:"column:state"`
	Attempts  int           `gorm:"column:attempts"`
	LastError *string       `gorm:"column:last_error"`
	CreatedAt time.Time     `gorm:"column:created_at"`
	UpdatedAt time.Time     `gorm:"column:updated_at"`
}

func (jobRecord) TableName() string { return "operation_jobs" }

func NewJobRepositoryWithDependencies(connections database.ConnectionProvider) (*JobRepository, error) {
	if connections == nil {
		return nil, ErrInvalidJobRepository
	}
	return &JobRepository{connections: connections}, nil
}

func (repository *JobRepository) connection(ctx context.Context) (*gormdb.DB, error) {
	if repository == nil || repository.connections == nil || ctx == nil {
		return nil, ErrInvalidJobRepository
	}
	return repository.connections.Connection(ctx)
}

func (repository *JobRepository) Create(ctx context.Context, job domain.Job) error {
	db, err := repository.connection(ctx)
	if err != nil {
		return err
	}
	id, err := databaseid.Encode(db.Dialector.Name(), job.ID)
	if err != nil {
		return err
	}
	record := map[string]any{"id": id, "name": job.Name, "payload": job.Payload, "state": string(job.State), "attempts": job.Attempts, "last_error": nil, "created_at": job.CreatedAt, "updated_at": job.UpdatedAt}
	if err := db.Table("operation_jobs").Create(record).Error; err != nil {
		return fmt.Errorf("create job: %w", err)
	}
	return nil
}

func (repository *JobRepository) List(ctx context.Context, limit int) ([]domain.Job, error) {
	db, err := repository.connection(ctx)
	if err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}
	var records []jobRecord
	if err := db.Order("created_at DESC, id DESC").Limit(limit).Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	items := make([]domain.Job, 0, len(records))
	for _, record := range records {
		item, mapErr := fromJobRecord(record)
		if mapErr != nil {
			return nil, mapErr
		}
		items = append(items, item)
	}
	return items, nil
}

func (repository *JobRepository) Find(ctx context.Context, id string) (domain.Job, error) {
	db, err := repository.connection(ctx)
	if err != nil {
		return domain.Job{}, err
	}
	encoded, err := databaseid.Encode(db.Dialector.Name(), id)
	if err != nil {
		return domain.Job{}, err
	}
	var record jobRecord
	if err := db.Where("id = ?", encoded).First(&record).Error; err != nil {
		if errors.Is(err, gormdb.ErrRecordNotFound) {
			return domain.Job{}, ports.ErrJobNotFound
		}
		return domain.Job{}, fmt.Errorf("find job: %w", err)
	}
	return fromJobRecord(record)
}

func (repository *JobRepository) Update(ctx context.Context, job domain.Job, expectedState domain.State) error {
	db, err := repository.connection(ctx)
	if err != nil {
		return err
	}
	encoded, err := databaseid.Encode(db.Dialector.Name(), job.ID)
	if err != nil {
		return err
	}
	result := db.Model(&jobRecord{}).Where("id = ? AND state = ?", encoded, string(expectedState)).Updates(map[string]any{"state": string(job.State), "attempts": job.Attempts, "last_error": nullableError(job.LastError), "updated_at": job.UpdatedAt})
	if result.Error != nil {
		return fmt.Errorf("update job: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ports.ErrJobNotFound
	}
	return nil
}

func nullableError(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func fromJobRecord(record jobRecord) (domain.Job, error) {
	if record.ID.String() == "" || record.Name == "" || len(record.Payload) == 0 || record.CreatedAt.IsZero() || record.UpdatedAt.IsZero() {
		return domain.Job{}, domain.ErrInvalidJob
	}
	lastError := ""
	if record.LastError != nil {
		lastError = *record.LastError
	}
	state := domain.State(record.State)
	if state != domain.StateQueued && state != domain.StateRunning && state != domain.StateFailed && state != domain.StateDone {
		return domain.Job{}, domain.ErrInvalidJob
	}
	return domain.Job{ID: record.ID.String(), Name: record.Name, Payload: append([]byte(nil), record.Payload...), State: state, Attempts: record.Attempts, LastError: lastError, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}, nil
}

var _ ports.JobRepository = (*JobRepository)(nil)
