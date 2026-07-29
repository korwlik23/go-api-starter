package application

import (
	"context"
	"errors"
	"strings"
	"time"

	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	localizationdomain "github.com/korwlik23/go-api-starter/internal/modules/localization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/localization/ports"
)

var (
	ErrInvalidLocaleManagement = errors.New("invalid locale management request")
	ErrLocaleManagementDenied  = errors.New("locale management denied")
)

type localeManagementAuthorizer interface {
	Allowed(
		ctx context.Context,
		userID string,
		permission authorizationdomain.PermissionKey,
	) (bool, error)
}

type ManageLocaleDependencies struct {
	Authorizer localeManagementAuthorizer
	Repository ports.LocaleManagementRepository
	NewID      func() (string, error)
	Now        func() time.Time
}

type CreateLocaleInput struct {
	Tag       string
	Name      string
	Direction string
}

type UpdateLocaleMetadataInput struct {
	Name            string
	Direction       string
	ExpectedVersion int64
}

type UpdateLocaleStatusInput struct {
	Enabled         bool
	UserSelectable  bool
	ExpectedVersion int64
}

type ManageLocales struct {
	dependencies ManageLocaleDependencies
	permission   authorizationdomain.PermissionKey
}

func NewManageLocales(
	dependencies ManageLocaleDependencies,
) (*ManageLocales, error) {
	permission, err := authorizationdomain.ParsePermissionKey(
		"localization.locales.manage.system",
	)
	if err != nil ||
		dependencies.Authorizer == nil ||
		dependencies.Repository == nil ||
		dependencies.NewID == nil ||
		dependencies.Now == nil {
		return nil, ErrInvalidLocaleManagement
	}
	return &ManageLocales{
		dependencies: dependencies,
		permission:   permission,
	}, nil
}

func (useCase *ManageLocales) Create(
	ctx context.Context,
	authenticatedUserID string,
	input CreateLocaleInput,
) (localizationdomain.Locale, error) {
	if err := useCase.authorize(ctx, authenticatedUserID); err != nil {
		return localizationdomain.Locale{}, err
	}
	tag, err := localizationdomain.NormalizeLocaleTag(input.Tag)
	direction, directionErr := parseDirection(input.Direction)
	name := strings.TrimSpace(input.Name)
	if err != nil ||
		directionErr != nil ||
		name == "" ||
		name != input.Name {
		return localizationdomain.Locale{}, ErrInvalidLocaleManagement
	}
	id, err := useCase.dependencies.NewID()
	if err != nil || strings.TrimSpace(id) == "" {
		return localizationdomain.Locale{}, ErrInvalidLocaleManagement
	}
	now := useCase.dependencies.Now().UTC()
	if now.IsZero() {
		return localizationdomain.Locale{}, ErrInvalidLocaleManagement
	}
	locale := localizationdomain.Locale{
		ID: id, Tag: tag, Name: name, Direction: direction,
		Enabled: false, UserSelectable: false, Default: false,
		Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := useCase.dependencies.Repository.Create(
		ctx,
		ports.CreateLocaleCommand{Locale: locale},
	); err != nil {
		return localizationdomain.Locale{}, err
	}
	return locale, nil
}

func (useCase *ManageLocales) UpdateMetadata(
	ctx context.Context,
	authenticatedUserID string,
	localeID string,
	input UpdateLocaleMetadataInput,
) (int64, error) {
	if err := useCase.authorize(ctx, authenticatedUserID); err != nil {
		return 0, err
	}
	current, err := useCase.dependencies.Repository.FindByID(ctx, localeID)
	if err != nil {
		return 0, err
	}
	direction, err := parseDirection(input.Direction)
	name := strings.TrimSpace(input.Name)
	if err != nil ||
		name == "" ||
		name != input.Name ||
		input.ExpectedVersion < 1 ||
		current.Version != input.ExpectedVersion {
		return 0, ErrInvalidLocaleManagement
	}
	return useCase.update(ctx, current, ports.UpdateLocaleCommand{
		ID: localeID, ExpectedVersion: input.ExpectedVersion,
		Name: name, Direction: direction,
		FallbackID: current.FallbackID, Enabled: current.Enabled,
		UserSelectable: current.UserSelectable,
	})
}

func (useCase *ManageLocales) UpdateStatus(
	ctx context.Context,
	authenticatedUserID string,
	localeID string,
	input UpdateLocaleStatusInput,
) (int64, error) {
	if err := useCase.authorize(ctx, authenticatedUserID); err != nil {
		return 0, err
	}
	current, err := useCase.dependencies.Repository.FindByID(ctx, localeID)
	if err != nil {
		return 0, err
	}
	if input.ExpectedVersion < 1 ||
		current.Version != input.ExpectedVersion ||
		(input.UserSelectable && !input.Enabled) {
		return 0, ErrInvalidLocaleManagement
	}
	return useCase.update(ctx, current, ports.UpdateLocaleCommand{
		ID: localeID, ExpectedVersion: input.ExpectedVersion,
		Name: current.Name, Direction: current.Direction,
		FallbackID: current.FallbackID, Enabled: input.Enabled,
		UserSelectable: input.UserSelectable,
	})
}

func (useCase *ManageLocales) SetDefault(
	ctx context.Context,
	authenticatedUserID string,
	localeID string,
	expectedVersion int64,
) (ports.DefaultLocaleResult, error) {
	if err := useCase.authorize(ctx, authenticatedUserID); err != nil {
		return ports.DefaultLocaleResult{}, err
	}
	if expectedVersion < 1 {
		return ports.DefaultLocaleResult{}, ErrInvalidLocaleManagement
	}
	return useCase.dependencies.Repository.SetDefault(
		ctx,
		localeID,
		expectedVersion,
		useCase.dependencies.Now().UTC(),
	)
}

func (useCase *ManageLocales) update(
	ctx context.Context,
	current localizationdomain.Locale,
	command ports.UpdateLocaleCommand,
) (int64, error) {
	command.UpdatedAt = useCase.dependencies.Now().UTC()
	if command.UpdatedAt.IsZero() ||
		current.ID == "" {
		return 0, ErrInvalidLocaleManagement
	}
	return useCase.dependencies.Repository.Update(ctx, command)
}

func (useCase *ManageLocales) authorize(
	ctx context.Context,
	authenticatedUserID string,
) error {
	if strings.TrimSpace(authenticatedUserID) == "" {
		return ErrLocaleManagementDenied
	}
	allowed, err := useCase.dependencies.Authorizer.Allowed(
		ctx,
		authenticatedUserID,
		useCase.permission,
	)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrLocaleManagementDenied
	}
	return nil
}

func parseDirection(value string) (localizationdomain.Direction, error) {
	direction := localizationdomain.Direction(value)
	if direction != localizationdomain.DirectionLTR &&
		direction != localizationdomain.DirectionRTL {
		return "", ErrInvalidLocaleManagement
	}
	return direction, nil
}
