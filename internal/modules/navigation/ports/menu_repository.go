package ports

import (
	"context"
	"errors"
	"regexp"
	"time"

	navigationdomain "github.com/korwlik23/go-api-starter/internal/modules/navigation/domain"
)

var (
	ErrInvalidMenuRepository = errors.New("invalid menu repository")
	ErrInvalidMenuQuery      = errors.New("invalid menu query")
	ErrMenuNotFound          = errors.New("menu not found")
	ErrMenuConflict          = errors.New("menu version conflict")
)

var menuIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type MenuCursor struct {
	UpdatedAt time.Time
	ID        string
}

type MenuListQuery struct {
	AccountID string
	LocaleID  string
	Enabled   *bool
	After     *MenuCursor
	Limit     int
}

type MenuListPage struct {
	Items []navigationdomain.Menu
	Next  *MenuCursor
}

type CreateMenuCommand struct {
	Menu  navigationdomain.Menu
	Items []navigationdomain.MenuItem
}

type UpdateMenuCommand struct {
	AccountID       string
	MenuID          string
	ExpectedVersion int64
	Menu            navigationdomain.Menu
	Items           []navigationdomain.MenuItem
}

type DeleteMenuCommand struct {
	AccountID       string
	MenuID          string
	ExpectedVersion int64
	DeletedAt       time.Time
}

type ReorderMenuCommand struct {
	AccountID       string
	MenuID          string
	ExpectedVersion int64
	Items           []navigationdomain.MenuItem
	UpdatedAt       time.Time
}

type MenuRepository interface {
	List(context.Context, MenuListQuery) (MenuListPage, error)
	Get(context.Context, string, string) (navigationdomain.Menu, error)
	FindPublic(context.Context, string, string, string) (navigationdomain.Menu, error)
	Create(context.Context, CreateMenuCommand) (navigationdomain.Menu, error)
	Update(context.Context, UpdateMenuCommand) (navigationdomain.Menu, error)
	Delete(context.Context, DeleteMenuCommand) error
	Reorder(context.Context, ReorderMenuCommand) (navigationdomain.Menu, error)
}

func ValidateMenuListQuery(query MenuListQuery) error {
	if !menuIDPattern.MatchString(query.AccountID) || query.Limit < 1 || query.Limit > 100 {
		return ErrInvalidMenuQuery
	}
	if query.LocaleID != "" && !menuIDPattern.MatchString(query.LocaleID) {
		return ErrInvalidMenuQuery
	}
	if query.After != nil && (query.After.UpdatedAt.IsZero() || !menuIDPattern.MatchString(query.After.ID)) {
		return ErrInvalidMenuQuery
	}
	return nil
}

func ValidateMenuIdentity(accountID, menuID string) error {
	if !menuIDPattern.MatchString(accountID) || !menuIDPattern.MatchString(menuID) {
		return ErrInvalidMenuQuery
	}
	return nil
}
