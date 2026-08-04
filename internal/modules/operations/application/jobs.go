package application

import (
	"context"
	"errors"
	"strings"
	"time"

	authapp "github.com/korwlik23/go-api-starter/internal/modules/authorization/application"
	authdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	operationsmodule "github.com/korwlik23/go-api-starter/internal/modules/operations"
	operationsdomain "github.com/korwlik23/go-api-starter/internal/modules/operations/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/operations/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
)

var (
	ErrInvalidJobService = errors.New("operations: invalid job service")
	ErrJobDenied         = errors.New("operations: job permission denied")
)

type PermissionEvaluator interface {
	Execute(context.Context, authapp.PermissionRequest) (bool, error)
}

type JobService struct {
	evaluator                        PermissionEvaluator
	repository                       ports.JobRepository
	transactions                     database.Transactor
	newID                            func() (string, error)
	now                              func() time.Time
	readPermission, managePermission authdomain.PermissionKey
}

func NewJobService(evaluator PermissionEvaluator, repository ports.JobRepository, transactions database.Transactor, newID func() (string, error)) (*JobService, error) {
	if evaluator == nil || repository == nil || newID == nil {
		return nil, ErrInvalidJobService
	}
	read, err := authdomain.ParsePermissionKey(operationsmodule.PermissionJobsReadSystem)
	if err != nil {
		return nil, ErrInvalidJobService
	}
	manage, err := authdomain.ParsePermissionKey(operationsmodule.PermissionJobsManageSystem)
	if err != nil {
		return nil, ErrInvalidJobService
	}
	return &JobService{evaluator: evaluator, repository: repository, transactions: transactions, newID: newID, now: time.Now, readPermission: read, managePermission: manage}, nil
}

func (service *JobService) List(ctx context.Context, actor string, limit int) ([]operationsdomain.Job, error) {
	if err := service.authorize(ctx, actor, service.readPermission); err != nil {
		return nil, err
	}
	return service.repository.List(ctx, limit)
}

func (service *JobService) Enqueue(ctx context.Context, actor, name string, payload []byte) (operationsdomain.Job, error) {
	if err := service.authorize(ctx, actor, service.managePermission); err != nil {
		return operationsdomain.Job{}, err
	}
	id, err := service.newID()
	if err != nil {
		return operationsdomain.Job{}, err
	}
	job, err := operationsdomain.NewJob(id, name, payload, service.now().UTC())
	if err != nil {
		return operationsdomain.Job{}, err
	}
	operation := func(operationContext context.Context) error { return service.repository.Create(operationContext, job) }
	if service.transactions != nil {
		err = service.transactions.WithinTransaction(ctx, operation)
	} else {
		err = operation(ctx)
	}
	if err != nil {
		return operationsdomain.Job{}, err
	}
	return job, nil
}

func (service *JobService) Retry(ctx context.Context, actor, id string) error {
	if err := service.authorize(ctx, actor, service.managePermission); err != nil {
		return err
	}
	operation := func(operationContext context.Context) error {
		job, err := service.repository.Find(operationContext, id)
		if err != nil {
			return err
		}
		expected := job.State
		if err := job.Retry(service.now().UTC()); err != nil {
			return err
		}
		return service.repository.Update(operationContext, job, expected)
	}
	if service.transactions != nil {
		return service.transactions.WithinTransaction(ctx, operation)
	}
	return operation(ctx)
}

func (service *JobService) authorize(ctx context.Context, actor string, permission authdomain.PermissionKey) error {
	if service == nil || ctx == nil || strings.TrimSpace(actor) == "" {
		return ErrInvalidJobService
	}
	allowed, err := service.evaluator.Execute(ctx, authapp.PermissionRequest{UserID: actor, Permission: permission})
	if err != nil {
		return err
	}
	if !allowed {
		return ErrJobDenied
	}
	return nil
}
