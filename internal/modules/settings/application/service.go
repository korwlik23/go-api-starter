package application

import (
	"context"
	"errors"
	"strings"
	"time"

	authdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/settings/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/settings/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
)

var (
	ErrInvalidService = errors.New("settings: invalid service")
	ErrDenied         = errors.New("settings: permission denied")
)

type Authorizer interface {
	Allowed(context.Context, string, authdomain.PermissionKey) (bool, error)
}

type Auditor interface {
	RecordSettingChange(context.Context, SettingAuditEvent) error
}

type SettingAuditEvent struct {
	ActorUserID string
	RequestID   string
	Key         string
	Version     int64
	Action      string
}

type Service struct {
	authorizer       Authorizer
	repository       ports.Repository
	transactions     database.Transactor
	auditor          Auditor
	now              func() time.Time
	readPermission   authdomain.PermissionKey
	managePermission authdomain.PermissionKey
}

func NewService(authorizer Authorizer, repository ports.Repository, transactions database.Transactor, auditor Auditor) (*Service, error) {
	if authorizer == nil || repository == nil {
		return nil, ErrInvalidService
	}
	read, err := authdomain.ParsePermissionKey("settings.global.read.system")
	if err != nil {
		return nil, ErrInvalidService
	}
	manage, err := authdomain.ParsePermissionKey("settings.global.manage.system")
	if err != nil {
		return nil, ErrInvalidService
	}
	return &Service{authorizer: authorizer, repository: repository, transactions: transactions, auditor: auditor, now: time.Now, readPermission: read, managePermission: manage}, nil
}

func (service *Service) List(ctx context.Context, actorUserID string) ([]domain.Setting, error) {
	if service == nil || ctx == nil || strings.TrimSpace(actorUserID) == "" {
		return nil, ErrInvalidService
	}
	if err := service.authorize(ctx, actorUserID, service.readPermission); err != nil {
		return nil, err
	}
	return service.repository.List(ctx)
}

func (service *Service) Update(ctx context.Context, actorUserID, key, value, requestID string, expectedVersion int64) (domain.Setting, error) {
	if service == nil || ctx == nil || strings.TrimSpace(actorUserID) == "" {
		return domain.Setting{}, ErrInvalidService
	}
	if err := service.authorize(ctx, actorUserID, service.managePermission); err != nil {
		return domain.Setting{}, err
	}
	key = strings.TrimSpace(key)
	if err := domain.ValidateKey(key); err != nil {
		return domain.Setting{}, err
	}
	if len(value) > 16384 {
		return domain.Setting{}, domain.ErrInvalidSetting
	}
	current, err := service.repository.Find(ctx, key)
	if expectedVersion == 0 {
		if err == nil {
			return domain.Setting{}, domain.ErrVersionConflict
		}
		if !errors.Is(err, ports.ErrNotFound) {
			return domain.Setting{}, err
		}
		current = domain.Setting{Key: key, Version: 1}
	} else {
		if err != nil {
			return domain.Setting{}, err
		}
		current, err = current.Update(key, value, expectedVersion)
		if err != nil {
			return domain.Setting{}, err
		}
	}
	if expectedVersion == 0 {
		current.Value = value
	}
	operation := func(operationContext context.Context) error {
		if err := service.repository.Save(operationContext, current, expectedVersion); err != nil {
			return err
		}
		if service.auditor == nil {
			return nil
		}
		return service.auditor.RecordSettingChange(operationContext, SettingAuditEvent{ActorUserID: actorUserID, RequestID: requestID, Key: key, Version: current.Version, Action: "settings.updated"})
	}
	if service.transactions != nil {
		if err := service.transactions.WithinTransaction(ctx, operation); err != nil {
			return domain.Setting{}, err
		}
	} else if err := operation(ctx); err != nil {
		return domain.Setting{}, err
	}
	return current, nil
}

func (service *Service) authorize(ctx context.Context, userID string, permission authdomain.PermissionKey) error {
	allowed, err := service.authorizer.Allowed(ctx, userID, permission)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrDenied
	}
	return nil
}
