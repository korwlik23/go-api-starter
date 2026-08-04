package application

import (
	"context"
	"errors"
	"strings"
	"time"

	authapp "github.com/korwlik23/go-api-starter/internal/modules/authorization/application"
	authdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/notifications/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/notifications/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
)

var (
	ErrInvalidService = errors.New("notifications: invalid service")
	ErrDenied         = errors.New("notifications: permission denied")
)

type PermissionEvaluator interface {
	Execute(context.Context, authapp.PermissionRequest) (bool, error)
}

type Service struct {
	evaluator        PermissionEvaluator
	repository       ports.Repository
	transactions     database.Transactor
	now              func() time.Time
	readPermission   authdomain.PermissionKey
	managePermission authdomain.PermissionKey
}

func NewService(evaluator PermissionEvaluator, repository ports.Repository, transactions database.Transactor) (*Service, error) {
	if evaluator == nil || repository == nil {
		return nil, ErrInvalidService
	}
	read, err := authdomain.ParsePermissionKey("notifications.inbox.read.own")
	if err != nil {
		return nil, ErrInvalidService
	}
	manage, err := authdomain.ParsePermissionKey("notifications.inbox.manage.own")
	if err != nil {
		return nil, ErrInvalidService
	}
	return &Service{evaluator: evaluator, repository: repository, transactions: transactions, now: time.Now, readPermission: read, managePermission: manage}, nil
}

func (service *Service) List(ctx context.Context, actorUserID string, limit int) ([]domain.Notification, error) {
	if err := service.authorize(ctx, actorUserID, service.readPermission); err != nil {
		return nil, err
	}
	return service.repository.ListForUser(ctx, actorUserID, limit)
}

func (service *Service) MarkRead(ctx context.Context, actorUserID, notificationID string) error {
	if err := service.authorize(ctx, actorUserID, service.managePermission); err != nil {
		return err
	}
	if err := service.transactionsOr(ctx, func(operationContext context.Context) error {
		current, err := service.repository.FindForUser(operationContext, actorUserID, notificationID)
		if err != nil {
			return err
		}
		if err := current.MarkRead(service.now().UTC()); err != nil {
			return err
		}
		return service.repository.MarkRead(operationContext, actorUserID, notificationID, current)
	}); err != nil {
		return err
	}
	return nil
}

func (service *Service) MarkAllRead(ctx context.Context, actorUserID string) error {
	if err := service.authorize(ctx, actorUserID, service.managePermission); err != nil {
		return err
	}
	now := service.now().UTC()
	return service.transactionsOr(ctx, func(operationContext context.Context) error {
		return service.repository.MarkAllRead(operationContext, actorUserID, domain.Notification{ReadAt: &now})
	})
}

func (service *Service) transactionsOr(ctx context.Context, operation func(context.Context) error) error {
	if service == nil || ctx == nil || operation == nil {
		return ErrInvalidService
	}
	if service.transactions != nil {
		return service.transactions.WithinTransaction(ctx, operation)
	}
	return operation(ctx)
}

func (service *Service) authorize(ctx context.Context, actorUserID string, permission authdomain.PermissionKey) error {
	if service == nil || ctx == nil || strings.TrimSpace(actorUserID) == "" {
		return ErrInvalidService
	}
	allowed, err := service.evaluator.Execute(ctx, authapp.PermissionRequest{UserID: actorUserID, Permission: permission, ResourceID: actorUserID})
	if err != nil {
		return err
	}
	if !allowed {
		return ErrDenied
	}
	return nil
}
