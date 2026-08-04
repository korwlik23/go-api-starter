package ports

import (
	"context"
	"github.com/korwlik23/go-api-starter/internal/modules/operations/domain"
)

var ErrJobNotFound = domain.ErrInvalidJob

type JobRepository interface {
	Create(context.Context, domain.Job) error
	List(context.Context, int) ([]domain.Job, error)
	Find(context.Context, string) (domain.Job, error)
	Update(context.Context, domain.Job, domain.State) error
}
