package menus

import (
	"context"
	"errors"
	"strings"
	"time"

	navigationdomain "github.com/korwlik23/go-api-starter/internal/modules/navigation/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/navigation/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
)

var (
	ErrInvalidMenuMutation = errors.New("navigation: invalid menu mutation")
	ErrInvalidMenuQuery    = errors.New("navigation: invalid menu query")
)

type MenuAuditEvent struct {
	ActorUserID string
	AccountID   string
	RequestID   string
	Action      string
	MenuID      string
	Version     int64
	Data        map[string]any
}

type MenuAuditor interface {
	RecordMenuChange(context.Context, MenuAuditEvent) error
}

type Dependencies struct {
	Repository   ports.MenuRepository
	Transactions database.Transactor
	Auditor      MenuAuditor
	NewID        func() (string, error)
	Now          func() time.Time
}

type Service struct {
	dependencies Dependencies
}

func NewService(dependencies Dependencies) (*Service, error) {
	if dependencies.Repository == nil || dependencies.Auditor == nil || dependencies.NewID == nil || dependencies.Now == nil {
		return nil, ErrInvalidMenuMutation
	}
	return &Service{dependencies: dependencies}, nil
}

type ItemInput struct {
	ID         string
	ParentID   string
	Label      string
	TargetKind navigationdomain.TargetKind
	Target     string
	Position   int64
	Enabled    bool
	Children   []ItemInput
}

type CreateInput struct {
	AccountID   string
	ActorUserID string
	RequestID   string
	LocaleID    string
	Key         string
	Name        string
	Enabled     bool
	Items       []ItemInput
}

type UpdateInput struct {
	AccountID       string
	ActorUserID     string
	RequestID       string
	MenuID          string
	ExpectedVersion int64
	LocaleID        string
	Key             string
	Name            string
	Enabled         bool
	Items           []ItemInput
}

type DeleteInput struct {
	AccountID       string
	ActorUserID     string
	RequestID       string
	MenuID          string
	ExpectedVersion int64
}

type ListInput struct {
	AccountID string
	LocaleID  string
	Enabled   *bool
	After     *ports.MenuCursor
	Limit     int
}

type ReorderItem struct {
	ID       string
	ParentID string
	Position int64
}

type ReorderInput struct {
	AccountID       string
	ActorUserID     string
	RequestID       string
	MenuID          string
	ExpectedVersion int64
	Items           []ReorderItem
}

func (service *Service) runMutation(ctx context.Context, operation func(context.Context) error) error {
	if service.dependencies.Transactions == nil {
		return operation(ctx)
	}
	return service.dependencies.Transactions.WithinTransaction(ctx, operation)
}

func (service *Service) materializeItems(
	accountID, menuID string,
	inputs []ItemInput,
	parentID string,
	now time.Time,
) ([]navigationdomain.MenuItem, error) {
	items := make([]navigationdomain.MenuItem, 0, len(inputs))
	for _, input := range inputs {
		id := strings.TrimSpace(input.ID)
		if id == "" {
			var err error
			id, err = service.dependencies.NewID()
			if err != nil {
				return nil, err
			}
		}
		effectiveParent := parentID
		if strings.TrimSpace(input.ParentID) != "" {
			if effectiveParent != "" && strings.TrimSpace(input.ParentID) != effectiveParent {
				return nil, ErrInvalidMenuMutation
			}
			effectiveParent = strings.TrimSpace(input.ParentID)
		}
		children, err := service.materializeItems(accountID, menuID, input.Children, id, now)
		if err != nil {
			return nil, err
		}
		item, err := navigationdomain.NewMenuItem(navigationdomain.MenuItemInput{
			ID: id, AccountID: accountID, MenuID: menuID, ParentID: effectiveParent,
			Label: input.Label, TargetKind: input.TargetKind, Target: input.Target,
			Position: input.Position, Enabled: input.Enabled, Version: 1,
			Children: children, CreatedAt: now, UpdatedAt: now,
		})
		if err != nil {
			return nil, ErrInvalidMenuMutation
		}
		items = append(items, item)
	}
	return items, nil
}

func (service *Service) buildMenu(
	id, accountID string,
	inputLocaleID, key, name string,
	enabled bool,
	version int64,
	items []ItemInput,
	now time.Time,
) (navigationdomain.Menu, []navigationdomain.MenuItem, error) {
	materialized, err := service.materializeItems(accountID, id, items, "", now)
	if err != nil {
		return navigationdomain.Menu{}, nil, err
	}
	if err := navigationdomain.ValidateTree(materialized, navigationdomain.MaxMenuDepth); err != nil {
		return navigationdomain.Menu{}, nil, ErrInvalidMenuMutation
	}
	menu, err := navigationdomain.NewMenu(navigationdomain.MenuInput{
		ID: id, AccountID: accountID, LocaleID: inputLocaleID, Key: key, Name: name,
		Enabled: enabled, Version: version, Items: materialized, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return navigationdomain.Menu{}, nil, ErrInvalidMenuMutation
	}
	return menu, navigationdomain.FlattenItems(materialized), nil
}
