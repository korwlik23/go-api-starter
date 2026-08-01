package gorm

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	accountsdomain "github.com/korwlik23/go-api-starter/internal/modules/accounts/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/accounts/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
	gormdb "gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrInvalidMembershipRepository = errors.New("invalid membership repository")

const (
	defaultMembershipPage = 25
	maxMembershipPage     = 100
)

type MembershipQueryRepository struct{ db *gormdb.DB }

type membershipQueryRecord struct {
	ID                   databaseid.ID `gorm:"column:id"`
	AccountID            databaseid.ID `gorm:"column:account_id"`
	AccountSlug          string        `gorm:"column:account_slug"`
	UserID               databaseid.ID `gorm:"column:user_id"`
	UserEmail            string        `gorm:"column:user_email"`
	Status               string        `gorm:"column:status"`
	AuthorizationVersion int64         `gorm:"column:authorization_version"`
	CreatedAt            time.Time     `gorm:"column:created_at"`
	UpdatedAt            time.Time     `gorm:"column:updated_at"`
}

func NewMembershipQueryRepository(db *gormdb.DB) (*MembershipQueryRepository, error) {
	if db == nil || db.Dialector == nil {
		return nil, ErrInvalidMembershipRepository
	}
	return &MembershipQueryRepository{db: db}, nil
}

func (repository *MembershipQueryRepository) ListMemberships(ctx context.Context, filter ports.MembershipListFilter) (ports.MembershipPage, error) {
	if repository == nil || repository.db == nil || ctx == nil {
		return ports.MembershipPage{}, ErrInvalidMembershipRepository
	}
	page, perPage, err := normalizeMembershipPage(filter.Page, filter.PerPage)
	if err != nil {
		return ports.MembershipPage{}, err
	}
	if strings.TrimSpace(filter.AccountID) == "" {
		return ports.MembershipPage{}, ports.ErrInvalidMembershipQuery
	}
	encodedAccountID, err := databaseid.Encode(repository.db.Dialector.Name(), strings.TrimSpace(filter.AccountID))
	if err != nil {
		return ports.MembershipPage{}, ports.ErrInvalidMembershipQuery
	}
	query := repository.db.WithContext(ctx).Table("memberships").
		Select("memberships.id, memberships.account_id, accounts.slug AS account_slug, memberships.user_id, users.normalized_email AS user_email, memberships.status, memberships.authorization_version, memberships.created_at, memberships.updated_at").
		Joins("JOIN accounts ON accounts.id = memberships.account_id").
		Joins("JOIN users ON users.id = memberships.user_id").
		Where("memberships.account_id = ?", encodedAccountID)
	if filter.Status != "" {
		if !validMembershipStatus(filter.Status) {
			return ports.MembershipPage{}, ports.ErrInvalidMembershipQuery
		}
		query = query.Where("memberships.status = ?", string(filter.Status))
	}
	countQuery := repository.db.WithContext(ctx).Table("memberships").Where("account_id = ?", encodedAccountID)
	if filter.Status != "" {
		countQuery = countQuery.Where("status = ?", string(filter.Status))
	}
	var total int64
	if err := countQuery.Count(&total).Error; err != nil {
		return ports.MembershipPage{}, fmt.Errorf("count memberships: %w", err)
	}
	var records []membershipQueryRecord
	if err := query.Order("memberships.created_at DESC").Order("memberships.id DESC").Limit(perPage).Offset((page - 1) * perPage).Find(&records).Error; err != nil {
		return ports.MembershipPage{}, fmt.Errorf("list memberships: %w", err)
	}
	items := make([]ports.MembershipView, 0, len(records))
	for _, record := range records {
		item, mapErr := mapMembershipRecord(record)
		if mapErr != nil {
			return ports.MembershipPage{}, mapErr
		}
		items = append(items, item)
	}
	return ports.MembershipPage{Items: items, Page: page, PerPage: perPage, TotalItems: clampMembershipInt(total), TotalPages: membershipTotalPages(total, perPage)}, nil
}

type MembershipCommandRepository struct {
	connections database.ConnectionProvider
	dialect     string
}

func NewMembershipCommandRepository(db *gormdb.DB) (*MembershipCommandRepository, error) {
	if db == nil || db.Dialector == nil {
		return nil, ErrInvalidMembershipRepository
	}
	return &MembershipCommandRepository{connections: directMembershipConnection{db: db}, dialect: db.Dialector.Name()}, nil
}

func NewTransactionalMembershipCommandRepository(connections database.ConnectionProvider, dialect string) (*MembershipCommandRepository, error) {
	if connections == nil || (dialect != "postgres" && dialect != "mysql") {
		return nil, ErrInvalidMembershipRepository
	}
	return &MembershipCommandRepository{connections: connections, dialect: dialect}, nil
}

func (repository *MembershipCommandRepository) UpdateMembershipStatus(ctx context.Context, accountID, membershipID string, expectedVersion int64, status accountsdomain.MembershipStatus, now time.Time) (ports.MembershipView, error) {
	db, err := repository.connection(ctx)
	if err != nil || expectedVersion < 1 || !validMembershipStatus(status) || now.IsZero() {
		if err != nil {
			return ports.MembershipView{}, err
		}
		return ports.MembershipView{}, ErrInvalidMembershipRepository
	}
	encodedAccountID, accountErr := databaseid.Encode(repository.dialect, strings.TrimSpace(accountID))
	encodedMembershipID, membershipErr := databaseid.Encode(repository.dialect, strings.TrimSpace(membershipID))
	if accountErr != nil || membershipErr != nil {
		return ports.MembershipView{}, ports.ErrMembershipNotFound
	}
	var record membershipQueryRecord
	findErr := db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Table("memberships").
		Select("memberships.id, memberships.account_id, accounts.slug AS account_slug, memberships.user_id, users.normalized_email AS user_email, memberships.status, memberships.authorization_version, memberships.created_at, memberships.updated_at").
		Joins("JOIN accounts ON accounts.id = memberships.account_id").Joins("JOIN users ON users.id = memberships.user_id").
		Where("memberships.account_id = ? AND memberships.id = ?", encodedAccountID, encodedMembershipID).First(&record).Error
	if errors.Is(findErr, gormdb.ErrRecordNotFound) {
		return ports.MembershipView{}, ports.ErrMembershipNotFound
	}
	if findErr != nil {
		return ports.MembershipView{}, fmt.Errorf("lock membership: %w", findErr)
	}
	if record.AuthorizationVersion != expectedVersion {
		return ports.MembershipView{}, ports.ErrMembershipConflict
	}
	if accountsdomain.MembershipStatus(record.Status) == status {
		return mapMembershipRecord(record)
	}
	result := db.WithContext(ctx).Table("memberships").Where("account_id = ? AND id = ? AND authorization_version = ?", encodedAccountID, encodedMembershipID, expectedVersion).
		Updates(map[string]any{"status": string(status), "authorization_version": expectedVersion + 1, "updated_at": now})
	if result.Error != nil {
		return ports.MembershipView{}, fmt.Errorf("update membership status: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ports.MembershipView{}, ports.ErrMembershipConflict
	}
	// Membership state affects effective permissions. Bump the principal version and
	// revoke every active session in the same transaction so stale cookies cannot be reused.
	if err := db.WithContext(ctx).Table("users").Where("id = ?", record.UserID).UpdateColumn("authorization_version", gormdb.Expr("authorization_version + 1")).Error; err != nil {
		return ports.MembershipView{}, fmt.Errorf("bump membership user version: %w", err)
	}
	if err := db.WithContext(ctx).Table("sessions").Where("user_id = ? AND revoked_at IS NULL", record.UserID).Update("revoked_at", now).Error; err != nil {
		return ports.MembershipView{}, fmt.Errorf("revoke membership sessions: %w", err)
	}
	record.Status = string(status)
	record.AuthorizationVersion = expectedVersion + 1
	record.UpdatedAt = now
	return mapMembershipRecord(record)
}

func (repository *MembershipCommandRepository) connection(ctx context.Context) (*gormdb.DB, error) {
	if repository == nil || repository.connections == nil || ctx == nil {
		return nil, ErrInvalidMembershipRepository
	}
	db, err := repository.connections.Connection(ctx)
	if err != nil || db == nil {
		if err != nil {
			return nil, err
		}
		return nil, ErrInvalidMembershipRepository
	}
	return db, nil
}

type directMembershipConnection struct{ db *gormdb.DB }

func (provider directMembershipConnection) Connection(ctx context.Context) (*gormdb.DB, error) {
	if provider.db == nil || ctx == nil {
		return nil, ErrInvalidMembershipRepository
	}
	return provider.db.WithContext(ctx), nil
}

func normalizeMembershipPage(page, perPage int) (int, int, error) {
	if page == 0 {
		page = 1
	}
	if perPage == 0 {
		perPage = defaultMembershipPage
	}
	if page < 1 || perPage < 1 || perPage > maxMembershipPage {
		return 0, 0, ports.ErrInvalidMembershipQuery
	}
	return page, perPage, nil
}
func validMembershipStatus(status accountsdomain.MembershipStatus) bool {
	return status == accountsdomain.MembershipStatusActive || status == accountsdomain.MembershipStatusDisabled
}
func mapMembershipRecord(record membershipQueryRecord) (ports.MembershipView, error) {
	item := ports.MembershipView{ID: record.ID.String(), AccountID: record.AccountID.String(), AccountSlug: record.AccountSlug, UserID: record.UserID.String(), UserEmail: record.UserEmail, Status: accountsdomain.MembershipStatus(record.Status), AuthorizationVersion: record.AuthorizationVersion, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}
	if item.ID == "" || item.AccountID == "" || item.UserID == "" || strings.TrimSpace(item.AccountSlug) == "" || strings.TrimSpace(item.UserEmail) == "" || !validMembershipStatus(item.Status) || item.AuthorizationVersion < 1 || item.CreatedAt.IsZero() || item.UpdatedAt.IsZero() || item.UpdatedAt.Before(item.CreatedAt) {
		return ports.MembershipView{}, ports.ErrInvalidMembershipQuery
	}
	return item, nil
}
func clampMembershipInt(value int64) int {
	if value < 0 {
		return 0
	}
	if value > int64(math.MaxInt) {
		return math.MaxInt
	}
	return int(value)
}
func membershipTotalPages(total int64, perPage int) int {
	if total <= 0 {
		return 0
	}
	return clampMembershipInt((total + int64(perPage) - 1) / int64(perPage))
}

var _ ports.MembershipQueryRepository = (*MembershipQueryRepository)(nil)
var _ ports.MembershipCommandRepository = (*MembershipCommandRepository)(nil)
