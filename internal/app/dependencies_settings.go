package app

import (
	"errors"

	"github.com/gin-gonic/gin"
	settingsmodule "github.com/korwlik23/go-api-starter/internal/modules/settings"
	settingsgorm "github.com/korwlik23/go-api-starter/internal/modules/settings/adapters/gorm"
	settingsapp "github.com/korwlik23/go-api-starter/internal/modules/settings/application"
	settingshttp "github.com/korwlik23/go-api-starter/internal/modules/settings/transport/http"
)

var errSettingsComposition = errors.New("app: settings composition failed")

type settingsRuntime struct {
	handler     *settingshttp.Handler
	readGuard   gin.HandlerFunc
	manageGuard gin.HandlerFunc
}

func buildSettingsRuntime(authorization authorizationRuntime) (settingsRuntime, error) {
	repository, err := settingsgorm.NewRepositoryWithDependencies(authorization.transactions)
	if err != nil {
		return settingsRuntime{}, err
	}
	service, err := settingsapp.NewService(authorization.systemAuthorizer, repository, authorization.transactions, authorizationAuditAdapter{appender: authorization.auditor})
	if err != nil {
		return settingsRuntime{}, err
	}
	handler, err := settingshttp.NewHandler(service)
	if err != nil {
		return settingsRuntime{}, err
	}
	readGuard, err := newPermissionGuard(authorization.accounts, authorization.evaluator, settingsmodule.PermissionReadSystem, nil)
	if err != nil {
		return settingsRuntime{}, err
	}
	manageGuard, err := newPermissionGuard(authorization.accounts, authorization.evaluator, settingsmodule.PermissionManageSystem, nil)
	if err != nil {
		return settingsRuntime{}, err
	}
	return settingsRuntime{handler: handler, readGuard: readGuard, manageGuard: manageGuard}, nil
}

var _ = errSettingsComposition
