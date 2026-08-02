package gorm

import (
	"context"
	"fmt"
	"time"

	navigationdomain "github.com/korwlik23/go-api-starter/internal/modules/navigation/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/navigation/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
	gormdb "gorm.io/gorm"
)

type MenuRepository struct {
	db          *gormdb.DB
	connections database.ConnectionProvider
}

type menuRecord struct {
	ID        databaseid.ID `gorm:"column:id"`
	AccountID databaseid.ID `gorm:"column:account_id"`
	LocaleID  databaseid.ID `gorm:"column:locale_id"`
	Key       string        `gorm:"column:menu_key"`
	Name      string        `gorm:"column:name"`
	Enabled   bool          `gorm:"column:enabled"`
	Version   int64         `gorm:"column:version"`
	DeletedAt *time.Time    `gorm:"column:deleted_at"`
	CreatedAt time.Time     `gorm:"column:created_at"`
	UpdatedAt time.Time     `gorm:"column:updated_at"`
}

type menuItemRecord struct {
	ID         databaseid.ID  `gorm:"column:id"`
	AccountID  databaseid.ID  `gorm:"column:account_id"`
	MenuID     databaseid.ID  `gorm:"column:menu_id"`
	ParentID   *databaseid.ID `gorm:"column:parent_id"`
	Label      string         `gorm:"column:label"`
	TargetKind string         `gorm:"column:target_kind"`
	Target     string         `gorm:"column:target"`
	Position   int64          `gorm:"column:position"`
	Enabled    bool           `gorm:"column:enabled"`
	Version    int64          `gorm:"column:version"`
	DeletedAt  *time.Time     `gorm:"column:deleted_at"`
	CreatedAt  time.Time      `gorm:"column:created_at"`
	UpdatedAt  time.Time      `gorm:"column:updated_at"`
}

func NewMenuRepository(db *gormdb.DB) (*MenuRepository, error) {
	if db == nil {
		return nil, ports.ErrInvalidMenuRepository
	}
	manager, err := database.NewTransactionManager(db)
	if err != nil {
		return nil, ports.ErrInvalidMenuRepository
	}
	return NewMenuRepositoryWithDependencies(db, manager), nil
}

func NewMenuRepositoryWithDependencies(db *gormdb.DB, connections database.ConnectionProvider) *MenuRepository {
	if db == nil || connections == nil {
		return nil
	}
	return &MenuRepository{db: db, connections: connections}
}

func (repository *MenuRepository) connection(ctx context.Context) (*gormdb.DB, error) {
	if repository == nil || repository.db == nil || repository.connections == nil || ctx == nil {
		return nil, ports.ErrInvalidMenuRepository
	}
	return repository.connections.Connection(ctx)
}

func (repository *MenuRepository) List(ctx context.Context, query ports.MenuListQuery) (ports.MenuListPage, error) {
	if repository == nil || repository.db == nil || ctx == nil {
		return ports.MenuListPage{}, ports.ErrInvalidMenuRepository
	}
	if err := ports.ValidateMenuListQuery(query); err != nil {
		return ports.MenuListPage{}, err
	}
	db, err := repository.connection(ctx)
	if err != nil {
		return ports.MenuListPage{}, err
	}
	accountID, err := databaseid.Encode(repository.db.Dialector.Name(), query.AccountID)
	if err != nil {
		return ports.MenuListPage{}, ports.ErrInvalidMenuQuery
	}
	db = db.WithContext(ctx).Table("navigation_menus").Select("id, account_id, locale_id, menu_key, name, enabled, version, deleted_at, created_at, updated_at").Where("account_id = ? AND deleted_at IS NULL", accountID)
	if query.LocaleID != "" {
		localeID, encodeErr := databaseid.Encode(repository.db.Dialector.Name(), query.LocaleID)
		if encodeErr != nil {
			return ports.MenuListPage{}, ports.ErrInvalidMenuQuery
		}
		db = db.Where("locale_id = ?", localeID)
	}
	if query.Enabled != nil {
		db = db.Where("enabled = ?", *query.Enabled)
	}
	if query.After != nil {
		cursorID, encodeErr := databaseid.Encode(repository.db.Dialector.Name(), query.After.ID)
		if encodeErr != nil {
			return ports.MenuListPage{}, ports.ErrInvalidMenuQuery
		}
		db = db.Where("(updated_at < ? OR (updated_at = ? AND id < ?))", query.After.UpdatedAt, query.After.UpdatedAt, cursorID)
	}
	var records []menuRecord
	if err := db.Order("updated_at DESC, id DESC").Limit(query.Limit + 1).Scan(&records).Error; err != nil {
		return ports.MenuListPage{}, fmt.Errorf("list navigation menus: %w", err)
	}
	page := ports.MenuListPage{Items: make([]navigationdomain.Menu, 0, query.Limit)}
	if len(records) > query.Limit {
		records = records[:query.Limit]
		last := records[len(records)-1]
		page.Next = &ports.MenuCursor{UpdatedAt: last.UpdatedAt.UTC(), ID: last.ID.String()}
	}
	for _, record := range records {
		menu, mapErr := menuFromRecord(record)
		if mapErr != nil {
			return ports.MenuListPage{}, mapErr
		}
		page.Items = append(page.Items, menu)
	}
	return page, nil
}

func (repository *MenuRepository) Get(ctx context.Context, accountID, menuID string) (navigationdomain.Menu, error) {
	return repository.get(ctx, accountID, menuID, false)
}

func (repository *MenuRepository) FindPublic(ctx context.Context, accountID, localeID, key string) (navigationdomain.Menu, error) {
	return repository.getPublic(ctx, accountID, localeID, key)
}

func (repository *MenuRepository) get(ctx context.Context, accountID, menuID string, publicOnly bool) (navigationdomain.Menu, error) {
	if repository == nil || repository.db == nil || ctx == nil {
		return navigationdomain.Menu{}, ports.ErrInvalidMenuRepository
	}
	if err := ports.ValidateMenuIdentity(accountID, menuID); err != nil {
		return navigationdomain.Menu{}, ports.ErrMenuNotFound
	}
	db, err := repository.connection(ctx)
	if err != nil {
		return navigationdomain.Menu{}, err
	}
	encodedAccount, err := databaseid.Encode(repository.db.Dialector.Name(), accountID)
	if err != nil {
		return navigationdomain.Menu{}, ports.ErrMenuNotFound
	}
	encodedMenu, err := databaseid.Encode(repository.db.Dialector.Name(), menuID)
	if err != nil {
		return navigationdomain.Menu{}, ports.ErrMenuNotFound
	}
	var record menuRecord
	menuQuery := db.WithContext(ctx).Table("navigation_menus").Select("id, account_id, locale_id, menu_key, name, enabled, version, deleted_at, created_at, updated_at").Where("account_id = ? AND id = ? AND deleted_at IS NULL", encodedAccount, encodedMenu)
	if publicOnly {
		menuQuery = menuQuery.Where("enabled = ?", true)
	}
	result := menuQuery.Limit(1).Scan(&record)
	if result.Error != nil {
		return navigationdomain.Menu{}, fmt.Errorf("get navigation menu: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return navigationdomain.Menu{}, ports.ErrMenuNotFound
	}
	menu, err := menuFromRecord(record)
	if err != nil {
		return navigationdomain.Menu{}, err
	}
	items, err := repository.items(ctx, encodedAccount, encodedMenu, publicOnly)
	if err != nil {
		return navigationdomain.Menu{}, err
	}
	menu.Items, err = navigationdomain.BuildTree(items)
	if err != nil {
		return navigationdomain.Menu{}, ports.ErrInvalidMenuRepository
	}
	return menu, nil
}

func (repository *MenuRepository) getPublic(ctx context.Context, accountID, localeID, key string) (navigationdomain.Menu, error) {
	if repository == nil || repository.db == nil || ctx == nil || key == "" {
		return navigationdomain.Menu{}, ports.ErrMenuNotFound
	}
	if !isMenuID(accountID) || !isMenuID(localeID) {
		return navigationdomain.Menu{}, ports.ErrMenuNotFound
	}
	db, err := repository.connection(ctx)
	if err != nil {
		return navigationdomain.Menu{}, err
	}
	encodedAccount, err := databaseid.Encode(repository.db.Dialector.Name(), accountID)
	if err != nil {
		return navigationdomain.Menu{}, ports.ErrMenuNotFound
	}
	encodedLocale, err := databaseid.Encode(repository.db.Dialector.Name(), localeID)
	if err != nil {
		return navigationdomain.Menu{}, ports.ErrMenuNotFound
	}
	var record menuRecord
	result := db.WithContext(ctx).Table("navigation_menus").Select("id, account_id, locale_id, menu_key, name, enabled, version, deleted_at, created_at, updated_at").Where("account_id = ? AND locale_id = ? AND menu_key = ? AND enabled = ? AND deleted_at IS NULL", encodedAccount, encodedLocale, key, true).Limit(1).Scan(&record)
	if result.Error != nil {
		return navigationdomain.Menu{}, fmt.Errorf("find public navigation menu: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return navigationdomain.Menu{}, ports.ErrMenuNotFound
	}
	menu, err := menuFromRecord(record)
	if err != nil {
		return navigationdomain.Menu{}, err
	}
	encodedMenu, err := databaseid.Encode(repository.db.Dialector.Name(), record.ID.String())
	if err != nil {
		return navigationdomain.Menu{}, ports.ErrMenuNotFound
	}
	items, err := repository.items(ctx, encodedAccount, encodedMenu, true)
	if err != nil {
		return navigationdomain.Menu{}, err
	}
	menu.Items, err = navigationdomain.BuildTree(items)
	if err != nil {
		return navigationdomain.Menu{}, ports.ErrInvalidMenuRepository
	}
	return menu, nil
}

func (repository *MenuRepository) items(ctx context.Context, accountID, menuID any, publicOnly bool) ([]navigationdomain.MenuItem, error) {
	db, err := repository.connection(ctx)
	if err != nil {
		return nil, err
	}
	var records []menuItemRecord
	query := db.WithContext(ctx).Table("navigation_menu_items").Select("id, account_id, menu_id, parent_id, label, target_kind, target, position, enabled, version, deleted_at, created_at, updated_at").Where("account_id = ? AND menu_id = ? AND deleted_at IS NULL", accountID, menuID)
	if publicOnly {
		query = query.Where("enabled = ?", true)
	}
	if err := query.Order("position ASC, id ASC").Scan(&records).Error; err != nil {
		return nil, fmt.Errorf("list navigation menu items: %w", err)
	}
	items := make([]navigationdomain.MenuItem, 0, len(records))
	for _, record := range records {
		item, mapErr := itemFromRecord(record)
		if mapErr != nil {
			return nil, mapErr
		}
		items = append(items, item)
	}
	return items, nil
}

func (repository *MenuRepository) Create(ctx context.Context, command ports.CreateMenuCommand) (navigationdomain.Menu, error) {
	db, err := repository.connection(ctx)
	if err != nil {
		return navigationdomain.Menu{}, err
	}
	menuID, accountID, localeID, err := repository.encodeMenuIDs(command.Menu.AccountID, command.Menu.ID, command.Menu.LocaleID)
	if err != nil {
		return navigationdomain.Menu{}, ports.ErrInvalidMenuRepository
	}
	if err := db.WithContext(ctx).Table("navigation_menus").Create(map[string]any{"id": menuID, "account_id": accountID, "locale_id": localeID, "menu_key": command.Menu.Key, "name": command.Menu.Name, "enabled": command.Menu.Enabled, "version": command.Menu.Version, "created_at": command.Menu.CreatedAt, "updated_at": command.Menu.UpdatedAt}).Error; err != nil {
		return navigationdomain.Menu{}, fmt.Errorf("create navigation menu: %w", err)
	}
	for _, item := range command.Items {
		if err := repository.createItem(ctx, db, item); err != nil {
			return navigationdomain.Menu{}, err
		}
	}
	return repository.Get(ctx, command.Menu.AccountID, command.Menu.ID)
}

func (repository *MenuRepository) Update(ctx context.Context, command ports.UpdateMenuCommand) (navigationdomain.Menu, error) {
	db, err := repository.connection(ctx)
	if err != nil {
		return navigationdomain.Menu{}, err
	}
	accountID, menuID, _, err := repository.encodeMenuIDs(command.AccountID, command.MenuID, command.Menu.LocaleID)
	if err != nil {
		return navigationdomain.Menu{}, ports.ErrMenuNotFound
	}
	localeID, err := databaseid.Encode(repository.db.Dialector.Name(), command.Menu.LocaleID)
	if err != nil {
		return navigationdomain.Menu{}, ports.ErrInvalidMenuRepository
	}
	result := db.WithContext(ctx).Table("navigation_menus").Where("account_id = ? AND id = ? AND version = ? AND deleted_at IS NULL", accountID, menuID, command.ExpectedVersion).Updates(map[string]any{"locale_id": localeID, "menu_key": command.Menu.Key, "name": command.Menu.Name, "enabled": command.Menu.Enabled, "version": command.Menu.Version, "updated_at": command.Menu.UpdatedAt})
	if result.Error != nil {
		return navigationdomain.Menu{}, fmt.Errorf("update navigation menu: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return navigationdomain.Menu{}, ports.ErrMenuConflict
	}
	if err := repository.replaceItems(ctx, db, accountID, menuID, command.Items, command.Menu.UpdatedAt); err != nil {
		return navigationdomain.Menu{}, err
	}
	return repository.Get(ctx, command.AccountID, command.MenuID)
}

func (repository *MenuRepository) Delete(ctx context.Context, command ports.DeleteMenuCommand) error {
	db, err := repository.connection(ctx)
	if err != nil {
		return err
	}
	accountID, menuID, err := repository.encodeAccountMenuIDs(command.AccountID, command.MenuID)
	if err != nil {
		return ports.ErrMenuNotFound
	}
	items := db.WithContext(ctx).Table("navigation_menu_items").Where("account_id = ? AND menu_id = ? AND deleted_at IS NULL", accountID, menuID).Updates(map[string]any{"deleted_at": command.DeletedAt, "updated_at": command.DeletedAt})
	if items.Error != nil {
		return fmt.Errorf("delete navigation menu items: %w", items.Error)
	}
	result := db.WithContext(ctx).Table("navigation_menus").Where("account_id = ? AND id = ? AND version = ? AND deleted_at IS NULL", accountID, menuID, command.ExpectedVersion).Updates(map[string]any{"deleted_at": command.DeletedAt, "version": command.ExpectedVersion + 1, "updated_at": command.DeletedAt})
	if result.Error != nil {
		return fmt.Errorf("delete navigation menu: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ports.ErrMenuConflict
	}
	return nil
}

func (repository *MenuRepository) Reorder(ctx context.Context, command ports.ReorderMenuCommand) (navigationdomain.Menu, error) {
	db, err := repository.connection(ctx)
	if err != nil {
		return navigationdomain.Menu{}, err
	}
	accountID, menuID, err := repository.encodeAccountMenuIDs(command.AccountID, command.MenuID)
	if err != nil {
		return navigationdomain.Menu{}, ports.ErrMenuNotFound
	}
	result := db.WithContext(ctx).Table("navigation_menus").Where("account_id = ? AND id = ? AND version = ? AND deleted_at IS NULL", accountID, menuID, command.ExpectedVersion).Updates(map[string]any{"version": command.ExpectedVersion + 1, "updated_at": command.UpdatedAt})
	if result.Error != nil {
		return navigationdomain.Menu{}, fmt.Errorf("reorder navigation menu: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return navigationdomain.Menu{}, ports.ErrMenuConflict
	}
	for _, item := range command.Items {
		parentID := any(nil)
		if item.ParentID != "" {
			encodedParent, encodeErr := databaseid.Encode(repository.db.Dialector.Name(), item.ParentID)
			if encodeErr != nil {
				return navigationdomain.Menu{}, ports.ErrMenuConflict
			}
			parentID = encodedParent
		}
		encodedItem, encodeErr := databaseid.Encode(repository.db.Dialector.Name(), item.ID)
		if encodeErr != nil {
			return navigationdomain.Menu{}, ports.ErrMenuConflict
		}
		itemResult := db.WithContext(ctx).Table("navigation_menu_items").Where("account_id = ? AND menu_id = ? AND id = ? AND deleted_at IS NULL", accountID, menuID, encodedItem).Updates(map[string]any{"parent_id": parentID, "position": item.Position, "version": gormdb.Expr("version + ?", 1), "updated_at": command.UpdatedAt})
		if itemResult.Error != nil {
			return navigationdomain.Menu{}, fmt.Errorf("reorder navigation menu item: %w", itemResult.Error)
		}
		if itemResult.RowsAffected != 1 {
			return navigationdomain.Menu{}, ports.ErrMenuConflict
		}
	}
	return repository.Get(ctx, command.AccountID, command.MenuID)
}

func (repository *MenuRepository) encodeMenuIDs(accountID, menuID, localeID string) (any, any, any, error) {
	account, err := databaseid.Encode(repository.db.Dialector.Name(), accountID)
	if err != nil {
		return nil, nil, nil, err
	}
	menu, err := databaseid.Encode(repository.db.Dialector.Name(), menuID)
	if err != nil {
		return nil, nil, nil, err
	}
	locale, err := databaseid.Encode(repository.db.Dialector.Name(), localeID)
	if err != nil {
		return nil, nil, nil, err
	}
	return account, menu, locale, nil
}

func (repository *MenuRepository) encodeAccountMenuIDs(accountID, menuID string) (any, any, error) {
	encodedAccount, encodedMenu, _, err := repository.encodeMenuIDs(accountID, menuID, accountID)
	return encodedAccount, encodedMenu, err
}

func (repository *MenuRepository) createItem(ctx context.Context, db *gormdb.DB, item navigationdomain.MenuItem) error {
	accountID, err := databaseid.Encode(repository.db.Dialector.Name(), item.AccountID)
	if err != nil {
		return ports.ErrInvalidMenuRepository
	}
	menuID, err := databaseid.Encode(repository.db.Dialector.Name(), item.MenuID)
	if err != nil {
		return ports.ErrInvalidMenuRepository
	}
	itemID, err := databaseid.Encode(repository.db.Dialector.Name(), item.ID)
	if err != nil {
		return ports.ErrInvalidMenuRepository
	}
	parentID := any(nil)
	if item.ParentID != "" {
		parentID, err = databaseid.Encode(repository.db.Dialector.Name(), item.ParentID)
		if err != nil {
			return ports.ErrInvalidMenuRepository
		}
	}
	if err := db.WithContext(ctx).Table("navigation_menu_items").Create(map[string]any{"id": itemID, "account_id": accountID, "menu_id": menuID, "parent_id": parentID, "label": item.Label, "target_kind": item.TargetKind, "target": item.Target, "position": item.Position, "enabled": item.Enabled, "version": item.Version, "created_at": item.CreatedAt, "updated_at": item.UpdatedAt}).Error; err != nil {
		return fmt.Errorf("create navigation menu item: %w", err)
	}
	return nil
}

func (repository *MenuRepository) replaceItems(ctx context.Context, db *gormdb.DB, accountID, menuID any, desired []navigationdomain.MenuItem, updatedAt time.Time) error {
	var existing []menuItemRecord
	if err := db.WithContext(ctx).Table("navigation_menu_items").Select("id, account_id, menu_id, parent_id, label, target_kind, target, position, enabled, version, deleted_at, created_at, updated_at").Where("account_id = ? AND menu_id = ? AND deleted_at IS NULL", accountID, menuID).Scan(&existing).Error; err != nil {
		return fmt.Errorf("load navigation menu items: %w", err)
	}
	existingByID := make(map[string]menuItemRecord, len(existing))
	for _, item := range existing {
		existingByID[item.ID.String()] = item
	}
	desiredIDs := make([]any, 0, len(desired))
	for _, item := range desired {
		encodedID, err := databaseid.Encode(repository.db.Dialector.Name(), item.ID)
		if err != nil {
			return ports.ErrInvalidMenuRepository
		}
		desiredIDs = append(desiredIDs, encodedID)
		if current, ok := existingByID[item.ID]; ok {
			parentID := any(nil)
			if item.ParentID != "" {
				parentID, err = databaseid.Encode(repository.db.Dialector.Name(), item.ParentID)
				if err != nil {
					return ports.ErrInvalidMenuRepository
				}
			}
			result := db.WithContext(ctx).Table("navigation_menu_items").Where("account_id = ? AND menu_id = ? AND id = ? AND deleted_at IS NULL", accountID, menuID, encodedID).Updates(map[string]any{"parent_id": parentID, "label": item.Label, "target_kind": item.TargetKind, "target": item.Target, "position": item.Position, "enabled": item.Enabled, "version": current.Version + 1, "updated_at": updatedAt})
			if result.Error != nil || result.RowsAffected != 1 {
				if result.Error != nil {
					return fmt.Errorf("update navigation menu item: %w", result.Error)
				}
				return ports.ErrMenuConflict
			}
			continue
		}
		item.CreatedAt = updatedAt
		item.UpdatedAt = updatedAt
		item.Version = 1
		if err := repository.createItem(ctx, db, item); err != nil {
			return err
		}
	}
	query := db.WithContext(ctx).Table("navigation_menu_items").Where("account_id = ? AND menu_id = ? AND deleted_at IS NULL", accountID, menuID)
	if len(desiredIDs) > 0 {
		query = query.Not("id IN ?", desiredIDs)
	}
	if result := query.Updates(map[string]any{"deleted_at": updatedAt, "updated_at": updatedAt}); result.Error != nil {
		return fmt.Errorf("delete navigation menu items: %w", result.Error)
	}
	return nil
}

func menuFromRecord(record menuRecord) (navigationdomain.Menu, error) {
	menu, err := navigationdomain.NewMenu(navigationdomain.MenuInput{ID: record.ID.String(), AccountID: record.AccountID.String(), LocaleID: record.LocaleID.String(), Key: record.Key, Name: record.Name, Enabled: record.Enabled, Version: record.Version, DeletedAt: record.DeletedAt, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt})
	if err != nil {
		return navigationdomain.Menu{}, ports.ErrInvalidMenuRepository
	}
	return menu, nil
}

func itemFromRecord(record menuItemRecord) (navigationdomain.MenuItem, error) {
	parentID := ""
	if record.ParentID != nil {
		parentID = record.ParentID.String()
	}
	item, err := navigationdomain.NewMenuItem(navigationdomain.MenuItemInput{ID: record.ID.String(), AccountID: record.AccountID.String(), MenuID: record.MenuID.String(), ParentID: parentID, Label: record.Label, TargetKind: navigationdomain.TargetKind(record.TargetKind), Target: record.Target, Position: record.Position, Enabled: record.Enabled, Version: record.Version, DeletedAt: record.DeletedAt, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt})
	if err != nil {
		return navigationdomain.MenuItem{}, ports.ErrInvalidMenuRepository
	}
	return item, nil
}

func isMenuID(value string) bool { return ports.ValidateMenuIdentity(value, value) == nil }

var _ ports.MenuRepository = (*MenuRepository)(nil)
