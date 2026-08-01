package gorm

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
	gormdb "gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrInvalidAdminUserRepository = errors.New("invalid admin user repository")

const (
	defaultAdminUserPage = 25
	maxAdminUserPage     = 100
)

type AdminUserQueryRepository struct{ db *gormdb.DB }

type adminUserRecord struct {
	ID                   databaseid.ID `gorm:"column:id"`
	NormalizedEmail      string        `gorm:"column:normalized_email"`
	Status               string        `gorm:"column:status"`
	AuthorizationVersion int64         `gorm:"column:authorization_version"`
	CreatedAt            time.Time     `gorm:"column:created_at"`
	UpdatedAt            time.Time     `gorm:"column:updated_at"`
	MembershipCount      int64         `gorm:"column:membership_count"`
	ActiveMemberships    int64         `gorm:"column:active_memberships"`
}

type adminMembershipRecord struct {
	ID                   databaseid.ID `gorm:"column:id"`
	AccountID            databaseid.ID `gorm:"column:account_id"`
	AccountSlug          string        `gorm:"column:account_slug"`
	Status               string        `gorm:"column:status"`
	AuthorizationVersion int64         `gorm:"column:authorization_version"`
	CreatedAt            time.Time     `gorm:"column:created_at"`
	UpdatedAt            time.Time     `gorm:"column:updated_at"`
}

func NewAdminUserQueryRepository(db *gormdb.DB) (*AdminUserQueryRepository, error) {
	if db == nil || db.Dialector == nil {
		return nil, ErrInvalidAdminUserRepository
	}
	return &AdminUserQueryRepository{db: db}, nil
}

func (repository *AdminUserQueryRepository) ListAdminUsers(
	ctx context.Context,
	filter ports.AdminUserListFilter,
) (ports.AdminUserPage, error) {
	if repository == nil || repository.db == nil || ctx == nil {
		return ports.AdminUserPage{}, ErrInvalidAdminUserRepository
	}
	page, perPage, err := normalizeAdminUserPage(filter.Page, filter.PerPage)
	if err != nil {
		return ports.AdminUserPage{}, err
	}
	query := repository.db.WithContext(ctx).Table("users").
		Select("users.id, users.normalized_email, users.status, users.authorization_version, users.created_at, users.updated_at, "+
			"(SELECT COUNT(*) FROM memberships WHERE memberships.user_id = users.id) AS membership_count, "+
			"(SELECT COUNT(*) FROM memberships WHERE memberships.user_id = users.id AND memberships.status = ?) AS active_memberships", string(identitydomain.UserStatusActive))
	if filter.Status != "" {
		if !validUserStatus(filter.Status) {
			return ports.AdminUserPage{}, ports.ErrInvalidAdminUserQuery
		}
		query = query.Where("users.status = ?", string(filter.Status))
	}
	if filter.Email != "" {
		email := identitydomain.NormalizeEmail(filter.Email)
		if email == "" || email != filter.Email && strings.TrimSpace(filter.Email) != filter.Email {
			return ports.AdminUserPage{}, ports.ErrInvalidAdminUserQuery
		}
		query = query.Where("users.normalized_email LIKE ?", email+"%")
	}
	var total int64
	countQuery := repository.db.WithContext(ctx).Table("users")
	if filter.Status != "" {
		countQuery = countQuery.Where("status = ?", string(filter.Status))
	}
	if filter.Email != "" {
		countQuery = countQuery.Where("normalized_email LIKE ?", identitydomain.NormalizeEmail(filter.Email)+"%")
	}
	if err := countQuery.Count(&total).Error; err != nil {
		return ports.AdminUserPage{}, fmt.Errorf("count admin users: %w", err)
	}
	var records []adminUserRecord
	if err := query.Order("users.created_at DESC").Order("users.id DESC").
		Limit(perPage).Offset((page - 1) * perPage).Find(&records).Error; err != nil {
		return ports.AdminUserPage{}, fmt.Errorf("list admin users: %w", err)
	}
	items := make([]ports.AdminUserSummary, 0, len(records))
	for _, record := range records {
		user, mapErr := mapAdminUserRecord(record)
		if mapErr != nil {
			return ports.AdminUserPage{}, mapErr
		}
		items = append(items, user)
	}
	return ports.AdminUserPage{Items: items, Page: page, PerPage: perPage,
		TotalItems: clampInt64(total), TotalPages: totalPages(total, perPage)}, nil
}

func (repository *AdminUserQueryRepository) FindAdminUser(
	ctx context.Context,
	userID string,
) (ports.AdminUserDetail, error) {
	if repository == nil || repository.db == nil || ctx == nil {
		return ports.AdminUserDetail{}, ErrInvalidAdminUserRepository
	}
	encodedID, err := databaseid.Encode(repository.db.Dialector.Name(), strings.TrimSpace(userID))
	if err != nil {
		return ports.AdminUserDetail{}, ports.ErrUserNotFound
	}
	var record adminUserRecord
	if err := repository.db.WithContext(ctx).Table("users").
		Select("id, normalized_email, status, authorization_version, created_at, updated_at").
		Where("id = ?", encodedID).First(&record).Error; errors.Is(err, gormdb.ErrRecordNotFound) {
		return ports.AdminUserDetail{}, ports.ErrUserNotFound
	} else if err != nil {
		return ports.AdminUserDetail{}, fmt.Errorf("find admin user: %w", err)
	}
	user := identitydomain.User{ID: record.ID.String(), Email: record.NormalizedEmail,
		Status: identitydomain.UserStatus(record.Status), AuthorizationVersion: record.AuthorizationVersion,
		CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}
	if user.ID == "" || !validUserStatus(user.Status) || user.AuthorizationVersion < 1 ||
		user.CreatedAt.IsZero() || user.UpdatedAt.IsZero() {
		return ports.AdminUserDetail{}, ports.ErrInvalidAdminUserQuery
	}
	var memberships []adminMembershipRecord
	if err := repository.db.WithContext(ctx).Table("memberships").
		Select("memberships.id, memberships.account_id, accounts.slug AS account_slug, memberships.status, memberships.authorization_version, memberships.created_at, memberships.updated_at").
		Joins("JOIN accounts ON accounts.id = memberships.account_id").
		Where("memberships.user_id = ?", encodedID).
		Order("memberships.created_at DESC").Order("memberships.id DESC").Find(&memberships).Error; err != nil {
		return ports.AdminUserDetail{}, fmt.Errorf("list user memberships: %w", err)
	}
	items := make([]ports.AdminUserMembership, 0, len(memberships))
	for _, membership := range memberships {
		if membership.ID.String() == "" || membership.AccountID.String() == "" ||
			strings.TrimSpace(membership.AccountSlug) == "" || !validMembershipStatus(membership.Status) ||
			membership.AuthorizationVersion < 1 || membership.CreatedAt.IsZero() || membership.UpdatedAt.IsZero() {
			return ports.AdminUserDetail{}, ports.ErrInvalidAdminUserQuery
		}
		items = append(items, ports.AdminUserMembership{ID: membership.ID.String(), AccountID: membership.AccountID.String(),
			AccountSlug: membership.AccountSlug, Status: membership.Status, AuthorizationVersion: membership.AuthorizationVersion,
			CreatedAt: membership.CreatedAt, UpdatedAt: membership.UpdatedAt})
	}
	return ports.AdminUserDetail{User: user, Memberships: items}, nil
}

type AdminUserCommandRepository struct {
	connections database.ConnectionProvider
	dialect     string
}

func NewAdminUserCommandRepository(db *gormdb.DB) (*AdminUserCommandRepository, error) {
	if db == nil || db.Dialector == nil {
		return nil, ErrInvalidAdminUserRepository
	}
	return &AdminUserCommandRepository{connections: directAdminUserConnection{db: db}, dialect: db.Dialector.Name()}, nil
}

func NewTransactionalAdminUserCommandRepository(connections database.ConnectionProvider, dialect string) (*AdminUserCommandRepository, error) {
	if connections == nil || (dialect != "postgres" && dialect != "mysql") {
		return nil, ErrInvalidAdminUserRepository
	}
	return &AdminUserCommandRepository{connections: connections, dialect: dialect}, nil
}

func (repository *AdminUserCommandRepository) UpdateUserStatus(
	ctx context.Context,
	userID string,
	expectedVersion int64,
	status identitydomain.UserStatus,
	now time.Time,
) (identitydomain.User, error) {
	db, err := repository.connection(ctx)
	if err != nil || expectedVersion < 1 || !validUserStatus(status) || now.IsZero() {
		return identitydomain.User{}, errOrInvalidAdminUser(err)
	}
	encodedID, encodeErr := databaseid.Encode(repository.dialect, strings.TrimSpace(userID))
	if encodeErr != nil {
		return identitydomain.User{}, ports.ErrUserNotFound
	}
	var current adminUserRecord
	findErr := db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Table("users").Select("id, normalized_email, status, authorization_version, created_at, updated_at").
		Where("id = ?", encodedID).First(&current).Error
	if errors.Is(findErr, gormdb.ErrRecordNotFound) {
		return identitydomain.User{}, ports.ErrUserNotFound
	}
	if findErr != nil {
		return identitydomain.User{}, fmt.Errorf("lock admin user: %w", findErr)
	}
	if current.AuthorizationVersion != expectedVersion {
		return identitydomain.User{}, ports.ErrAdminUserConflict
	}
	if identitydomain.UserStatus(current.Status) == status {
		return mapAdminUser(current)
	}
	result := db.WithContext(ctx).Table("users").Where("id = ? AND authorization_version = ?", encodedID, expectedVersion).
		Updates(map[string]any{"status": string(status), "authorization_version": expectedVersion + 1, "updated_at": now})
	if result.Error != nil {
		return identitydomain.User{}, fmt.Errorf("update admin user status: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return identitydomain.User{}, ports.ErrAdminUserConflict
	}
	if revokeErr := db.WithContext(ctx).Table("sessions").Where("user_id = ? AND revoked_at IS NULL", encodedID).Update("revoked_at", now).Error; revokeErr != nil {
		return identitydomain.User{}, fmt.Errorf("revoke user sessions: %w", revokeErr)
	}
	current.Status = string(status)
	current.AuthorizationVersion = expectedVersion + 1
	current.UpdatedAt = now
	return mapAdminUser(current)
}

func (repository *AdminUserCommandRepository) connection(ctx context.Context) (*gormdb.DB, error) {
	if repository == nil || repository.connections == nil || ctx == nil {
		return nil, ErrInvalidAdminUserRepository
	}
	db, err := repository.connections.Connection(ctx)
	if err != nil || db == nil {
		return nil, errOrInvalidAdminUser(err)
	}
	return db, nil
}

type directAdminUserConnection struct{ db *gormdb.DB }

func (provider directAdminUserConnection) Connection(ctx context.Context) (*gormdb.DB, error) {
	if provider.db == nil || ctx == nil {
		return nil, ErrInvalidAdminUserRepository
	}
	return provider.db.WithContext(ctx), nil
}

func normalizeAdminUserPage(page, perPage int) (int, int, error) {
	if page == 0 {
		page = 1
	}
	if perPage == 0 {
		perPage = defaultAdminUserPage
	}
	if page < 1 || perPage < 1 || perPage > maxAdminUserPage {
		return 0, 0, ports.ErrInvalidAdminUserQuery
	}
	return page, perPage, nil
}

func mapAdminUserRecord(record adminUserRecord) (ports.AdminUserSummary, error) {
	user, err := mapAdminUser(record)
	if err != nil {
		return ports.AdminUserSummary{}, err
	}
	return ports.AdminUserSummary{User: user, MembershipCount: clampInt64(record.MembershipCount), ActiveMembershipCount: clampInt64(record.ActiveMemberships)}, nil
}

func mapAdminUser(record adminUserRecord) (identitydomain.User, error) {
	user := identitydomain.User{ID: record.ID.String(), Email: record.NormalizedEmail, Status: identitydomain.UserStatus(record.Status), AuthorizationVersion: record.AuthorizationVersion, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}
	if user.ID == "" || strings.TrimSpace(user.Email) == "" || !validUserStatus(user.Status) || user.AuthorizationVersion < 1 || user.CreatedAt.IsZero() || user.UpdatedAt.IsZero() || user.UpdatedAt.Before(user.CreatedAt) {
		return identitydomain.User{}, ports.ErrInvalidAdminUserQuery
	}
	return user, nil
}

func validUserStatus(status identitydomain.UserStatus) bool {
	return status == identitydomain.UserStatusActive || status == identitydomain.UserStatusDisabled
}
func validMembershipStatus(status string) bool { return status == "active" || status == "disabled" }
func clampInt64(value int64) int {
	if value < 0 {
		return 0
	}
	if value > int64(math.MaxInt) {
		return math.MaxInt
	}
	return int(value)
}
func totalPages(total int64, perPage int) int {
	if total <= 0 {
		return 0
	}
	pages := (total + int64(perPage) - 1) / int64(perPage)
	return clampInt64(pages)
}
func errOrInvalidAdminUser(err error) error {
	if err != nil {
		return err
	}
	return ErrInvalidAdminUserRepository
}

var _ ports.AdminUserQueryRepository = (*AdminUserQueryRepository)(nil)
var _ ports.AdminUserCommandRepository = (*AdminUserCommandRepository)(nil)
