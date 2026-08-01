package application

import (
	"context"
	"errors"
	"strings"
	"time"

	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	localizationdomain "github.com/korwlik23/go-api-starter/internal/modules/localization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/localization/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
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

type LocaleManagementAuditEvent struct {
	ActorUserID           string
	RequestID             string
	Action                string
	LocaleID              string
	Version               int64
	PreviousDefaultLocale string
}

type LocaleManagementAuditor interface {
	RecordLocaleChange(context.Context, LocaleManagementAuditEvent) error
}

type ManageLocaleDependencies struct {
	Authorizer   localeManagementAuthorizer
	Repository   ports.LocaleManagementRepository
	NewID        func() (string, error)
	Now          func() time.Time
	Transactions database.Transactor
	Auditor      LocaleManagementAuditor
}

type CreateLocaleInput struct {
	Tag       string
	Name      string
	Direction string
	RequestID string
}

type UpdateLocaleMetadataInput struct {
	Name            string
	Direction       string
	ExpectedVersion int64
	RequestID       string
}

type UpdateLocaleStatusInput struct {
	Enabled         bool
	UserSelectable  bool
	ExpectedVersion int64
	RequestID       string
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
	var operationErr error
	operation := func(operationContext context.Context) error {
		if err := useCase.dependencies.Repository.Create(
			operationContext,
			ports.CreateLocaleCommand{Locale: locale},
		); err != nil {
			return err
		}
		if useCase.dependencies.Auditor == nil {
			return nil
		}
		return useCase.dependencies.Auditor.RecordLocaleChange(
			operationContext,
			LocaleManagementAuditEvent{
				ActorUserID: authenticatedUserID, RequestID: input.RequestID,
				Action: "localization.locale.created", LocaleID: locale.ID,
				Version: locale.Version,
			},
		)
	}
	if useCase.dependencies.Transactions != nil {
		operationErr = useCase.dependencies.Transactions.WithinTransaction(ctx, operation)
	} else {
		operationErr = operation(ctx)
	}
	if operationErr != nil {
		return localizationdomain.Locale{}, operationErr
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
	var version int64
	operation := func(operationContext context.Context) error {
		var updateErr error
		version, updateErr = useCase.update(operationContext, current, ports.UpdateLocaleCommand{
			ID: localeID, ExpectedVersion: input.ExpectedVersion,
			Name: name, Direction: direction,
			FallbackID: current.FallbackID, Enabled: current.Enabled,
			UserSelectable: current.UserSelectable,
		})
		if updateErr != nil || useCase.dependencies.Auditor == nil {
			return updateErr
		}
		return useCase.dependencies.Auditor.RecordLocaleChange(operationContext, LocaleManagementAuditEvent{
			ActorUserID: authenticatedUserID, RequestID: input.RequestID,
			Action: "localization.locale.metadata.updated", LocaleID: localeID,
			Version: version,
		})
	}
	return version, useCase.runMutation(ctx, operation)
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
	var version int64
	operation := func(operationContext context.Context) error {
		var updateErr error
		version, updateErr = useCase.update(operationContext, current, ports.UpdateLocaleCommand{
			ID: localeID, ExpectedVersion: input.ExpectedVersion,
			Name: current.Name, Direction: current.Direction,
			FallbackID: current.FallbackID, Enabled: input.Enabled,
			UserSelectable: input.UserSelectable,
		})
		if updateErr != nil || useCase.dependencies.Auditor == nil {
			return updateErr
		}
		return useCase.dependencies.Auditor.RecordLocaleChange(operationContext, LocaleManagementAuditEvent{
			ActorUserID: authenticatedUserID, RequestID: input.RequestID,
			Action: "localization.locale.status.updated", LocaleID: localeID,
			Version: version,
		})
	}
	return version, useCase.runMutation(ctx, operation)
}

func (useCase *ManageLocales) SetDefault(
	ctx context.Context,
	authenticatedUserID string,
	localeID string,
	expectedVersion int64,
	requestID string,
) (ports.DefaultLocaleResult, error) {
	if err := useCase.authorize(ctx, authenticatedUserID); err != nil {
		return ports.DefaultLocaleResult{}, err
	}
	if expectedVersion < 1 {
		return ports.DefaultLocaleResult{}, ErrInvalidLocaleManagement
	}
	var result ports.DefaultLocaleResult
	operation := func(operationContext context.Context) error {
		var err error
		result, err = useCase.dependencies.Repository.SetDefault(
			operationContext, localeID, expectedVersion,
			useCase.dependencies.Now().UTC(),
		)
		if err != nil || useCase.dependencies.Auditor == nil {
			return err
		}
		return useCase.dependencies.Auditor.RecordLocaleChange(
			operationContext,
			LocaleManagementAuditEvent{
				ActorUserID: authenticatedUserID,
				RequestID:   requestID,
				Action:      "localization.locale.default.updated", LocaleID: localeID,
				Version:               result.Version,
				PreviousDefaultLocale: result.PreviousDefaultLocaleID,
			},
		)
	}
	var err error
	if useCase.dependencies.Transactions != nil {
		err = useCase.dependencies.Transactions.WithinTransaction(ctx, operation)
	} else {
		err = operation(ctx)
	}
	return result, err
}

func (useCase *ManageLocales) runMutation(
	ctx context.Context,
	operation func(context.Context) error,
) error {
	if useCase.dependencies.Transactions == nil {
		return operation(ctx)
	}
	return useCase.dependencies.Transactions.WithinTransaction(ctx, operation)
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
