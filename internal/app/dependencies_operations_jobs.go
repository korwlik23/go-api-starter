package app

import (
	"github.com/gin-gonic/gin"
	operationsmodule "github.com/korwlik23/go-api-starter/internal/modules/operations"
	operationsgorm "github.com/korwlik23/go-api-starter/internal/modules/operations/adapters/gorm"
	operationsapp "github.com/korwlik23/go-api-starter/internal/modules/operations/application"
	operationshttp "github.com/korwlik23/go-api-starter/internal/modules/operations/transport/http"
)

type operationsJobsRuntime struct {
	handler     *operationshttp.JobHandler
	readGuard   gin.HandlerFunc
	manageGuard gin.HandlerFunc
}

func buildOperationsJobsRuntime(authorization authorizationRuntime) (operationsJobsRuntime, error) {
	repository, err := operationsgorm.NewJobRepositoryWithDependencies(authorization.transactions)
	if err != nil {
		return operationsJobsRuntime{}, err
	}
	service, err := operationsapp.NewJobService(authorization.evaluator, repository, authorization.transactions, newRuntimeID)
	if err != nil {
		return operationsJobsRuntime{}, err
	}
	handler, err := operationshttp.NewJobHandler(service)
	if err != nil {
		return operationsJobsRuntime{}, err
	}
	read, err := newPermissionGuard(authorization.accounts, authorization.evaluator, operationsmodule.PermissionJobsReadSystem, nil)
	if err != nil {
		return operationsJobsRuntime{}, err
	}
	manage, err := newPermissionGuard(authorization.accounts, authorization.evaluator, operationsmodule.PermissionJobsManageSystem, nil)
	if err != nil {
		return operationsJobsRuntime{}, err
	}
	return operationsJobsRuntime{handler: handler, readGuard: read, manageGuard: manage}, nil
}
