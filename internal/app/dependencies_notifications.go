package app

import (
	"github.com/gin-gonic/gin"
	notificationsmodule "github.com/korwlik23/go-api-starter/internal/modules/notifications"
	notificationsgorm "github.com/korwlik23/go-api-starter/internal/modules/notifications/adapters/gorm"
	notificationsapp "github.com/korwlik23/go-api-starter/internal/modules/notifications/application"
	notificationshttp "github.com/korwlik23/go-api-starter/internal/modules/notifications/transport/http"
)

type notificationsRuntime struct {
	handler     *notificationshttp.Handler
	readGuard   gin.HandlerFunc
	manageGuard gin.HandlerFunc
}

func buildNotificationsRuntime(authorization authorizationRuntime) (notificationsRuntime, error) {
	repository, err := notificationsgorm.NewRepositoryWithDependencies(authorization.transactions)
	if err != nil {
		return notificationsRuntime{}, err
	}
	service, err := notificationsapp.NewService(authorization.evaluator, repository, authorization.transactions)
	if err != nil {
		return notificationsRuntime{}, err
	}
	handler, err := notificationshttp.NewHandler(service)
	if err != nil {
		return notificationsRuntime{}, err
	}
	readGuard, err := newPermissionGuard(authorization.accounts, authorization.evaluator, notificationsmodule.PermissionReadOwn, currentPrincipalResourceID)
	if err != nil {
		return notificationsRuntime{}, err
	}
	manageGuard, err := newPermissionGuard(authorization.accounts, authorization.evaluator, notificationsmodule.PermissionManageOwn, currentPrincipalResourceID)
	if err != nil {
		return notificationsRuntime{}, err
	}
	return notificationsRuntime{handler: handler, readGuard: readGuard, manageGuard: manageGuard}, nil
}
