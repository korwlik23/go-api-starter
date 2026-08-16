package seed

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
	"gorm.io/gorm"
)

type store struct {
	db      *gorm.DB
	dialect string
	ids     IDs
}

func newStore(db *gorm.DB, ids IDs) (*store, error) {
	if db == nil {
		return nil, ErrInvalidConfig
	}
	dialect := db.Dialector.Name()
	if dialect != "postgres" && dialect != "mysql" {
		return nil, fmt.Errorf("seed: unsupported database dialect %q", dialect)
	}
	return &store{db: db, dialect: dialect, ids: ids}, nil
}

func (s *store) ensure(
	ctx context.Context,
	table string,
	where map[string]any,
	values map[string]any,
) error {
	if s == nil || s.db == nil || strings.TrimSpace(table) == "" {
		return ErrInvalidConfig
	}
	query := s.db.WithContext(ctx).Table(table)
	encodedValues, err := s.encodeMap(values)
	if err != nil {
		return err
	}
	var count int64
	countQuery, err := s.where(query, where)
	if err != nil {
		return err
	}
	if err := countQuery.Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return query.Create(encodedValues).Error
	}
	return s.update(ctx, table, where, encodedValues)
}

func (s *store) link(ctx context.Context, table string, values map[string]any) error {
	where := make(map[string]any, len(values))
	for key, value := range values {
		where[key] = value
	}
	return s.ensure(ctx, table, where, values)
}

func (s *store) findID(
	ctx context.Context,
	table string,
	where map[string]any,
) (string, error) {
	query := s.db.WithContext(ctx).Table(table).Select("id")
	query, err := s.where(query, where)
	if err != nil {
		return "", err
	}
	var row struct {
		ID databaseid.ID `gorm:"column:id"`
	}
	result := query.Take(&row)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return "", gorm.ErrRecordNotFound
	}
	if result.Error != nil {
		return "", result.Error
	}
	return row.ID.String(), nil
}

func (s *store) where(query *gorm.DB, values map[string]any) (*gorm.DB, error) {
	encoded, err := s.encodeMap(values)
	if err != nil {
		return nil, err
	}
	for key, value := range encoded {
		column := s.quoteColumn(key)
		if value == nil {
			query = query.Where(column + " IS NULL")
			continue
		}
		query = query.Where(column+" = ?", value)
	}
	return query, nil
}

func (s *store) quoteColumn(column string) string {
	if s.dialect == "mysql" {
		return "`" + column + "`"
	}
	return `"` + column + `"`
}

func (s *store) quoteTable(table string) string {
	if s.dialect == "mysql" {
		return "`" + table + "`"
	}
	return `"` + table + `"`
}

func (s *store) update(
	ctx context.Context,
	table string,
	where map[string]any,
	values map[string]any,
) error {
	encodedWhere, err := s.encodeMap(where)
	if err != nil {
		return err
	}
	encodedValues, err := s.encodeMap(values)
	if err != nil {
		return err
	}
	if len(encodedWhere) == 0 || len(encodedValues) == 0 {
		return ErrInvalidConfig
	}
	valueKeys := sortedKeys(encodedValues)
	whereKeys := sortedKeys(encodedWhere)
	assignments := make([]string, 0, len(valueKeys))
	args := make([]any, 0, len(valueKeys)+len(whereKeys))
	for _, key := range valueKeys {
		assignments = append(assignments, s.quoteColumn(key)+" = ?")
		args = append(args, encodedValues[key])
	}
	conditions := make([]string, 0, len(whereKeys))
	for _, key := range whereKeys {
		value := encodedWhere[key]
		if value == nil {
			conditions = append(conditions, s.quoteColumn(key)+" IS NULL")
			continue
		}
		conditions = append(conditions, s.quoteColumn(key)+" = ?")
		args = append(args, value)
	}
	statement := "UPDATE " + s.quoteTable(table) +
		" SET " + strings.Join(assignments, ", ") +
		" WHERE " + strings.Join(conditions, " AND ")
	return s.db.WithContext(ctx).Exec(statement, args...).Error
}

func sortedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (s *store) encodeMap(values map[string]any) (map[string]any, error) {
	encoded := make(map[string]any, len(values))
	for key, value := range values {
		if value == nil || !isUUIDColumn(key) {
			encoded[key] = value
			continue
		}
		text, ok := value.(string)
		if !ok || strings.TrimSpace(text) == "" {
			encoded[key] = value
			continue
		}
		converted, err := databaseid.Encode(s.dialect, text)
		if err != nil {
			return nil, err
		}
		encoded[key] = converted
	}
	return encoded, nil
}

func isUUIDColumn(column string) bool {
	switch column {
	case "id", "account_id", "user_id", "membership_id", "role_id", "permission_id", "assignment_id", "locale_id", "fallback_locale_id", "content_id", "translation_id", "taxonomy_id", "author_id", "actor_id", "menu_id", "parent_id", "asset_id", "operation_id":
		return true
	default:
		return false
	}
}

func idValue(value string, dialect string) (driver.Value, error) {
	return databaseid.Encode(dialect, value)
}
