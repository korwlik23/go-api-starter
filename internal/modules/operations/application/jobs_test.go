package application

import (
	"context"
	"fmt"
	"testing"
	"time"

	authapp "github.com/korwlik23/go-api-starter/internal/modules/authorization/application"
	"github.com/korwlik23/go-api-starter/internal/modules/operations/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/operations/ports"
	"github.com/stretchr/testify/require"
)

type jobEvaluatorFake struct{ allowed bool }

func (fake jobEvaluatorFake) Execute(context.Context, authapp.PermissionRequest) (bool, error) {
	return fake.allowed, nil
}

type jobRepositoryFake struct{ jobs map[string]domain.Job }

func (fake *jobRepositoryFake) Create(_ context.Context, job domain.Job) error {
	if fake.jobs == nil {
		fake.jobs = map[string]domain.Job{}
	}
	fake.jobs[job.ID] = job
	return nil
}
func (fake *jobRepositoryFake) List(context.Context, int) ([]domain.Job, error) {
	items := make([]domain.Job, 0, len(fake.jobs))
	for _, job := range fake.jobs {
		items = append(items, job)
	}
	return items, nil
}
func (fake *jobRepositoryFake) Find(_ context.Context, id string) (domain.Job, error) {
	job, ok := fake.jobs[id]
	if !ok {
		return domain.Job{}, ports.ErrJobNotFound
	}
	return job, nil
}
func (fake *jobRepositoryFake) Update(_ context.Context, job domain.Job, expected domain.State) error {
	current, ok := fake.jobs[job.ID]
	if !ok || current.State != expected {
		return ports.ErrJobNotFound
	}
	fake.jobs[job.ID] = job
	return nil
}

func TestJobServiceEnqueuesAndRetriesFailedJob(t *testing.T) {
	repository := &jobRepositoryFake{}
	idCounter := 0
	service, err := NewJobService(jobEvaluatorFake{allowed: true}, repository, nil, func() (string, error) {
		idCounter++
		return fmt.Sprintf("00000000-0000-0000-0000-00000000000%d", idCounter), nil
	})
	require.NoError(t, err)
	job, err := service.Enqueue(context.Background(), "admin", "media.cleanup", []byte(`{"limit":10}`))
	require.NoError(t, err)
	require.Equal(t, domain.StateQueued, job.State)
	failed := job
	require.NoError(t, failed.Fail("storage unavailable", time.Now().UTC()))
	repository.jobs[job.ID] = failed
	require.NoError(t, service.Retry(context.Background(), "admin", job.ID))
	require.Equal(t, domain.StateQueued, repository.jobs[job.ID].State)
}

func TestJobServiceDeniesQueueWithoutPermission(t *testing.T) {
	service, err := NewJobService(jobEvaluatorFake{allowed: false}, &jobRepositoryFake{}, nil, func() (string, error) { return "00000000-0000-0000-0000-000000000001", nil })
	require.NoError(t, err)
	_, err = service.Enqueue(context.Background(), "user", "media.cleanup", []byte(`{}`))
	require.ErrorIs(t, err, ErrJobDenied)
}
