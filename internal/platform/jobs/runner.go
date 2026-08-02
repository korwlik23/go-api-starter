package jobs

import (
	"context"
	"errors"
)

var (
	ErrInvalidRunner = errors.New("jobs: invalid runner")
	ErrDuplicateJob  = errors.New("jobs: duplicate job")
)

type Job interface {
	ID() string
	Run(context.Context) error
}

type Result struct {
	ID  string
	Err error
}

type Runner struct{ jobs []Job }

func NewRunner(jobs ...Job) (*Runner, error) {
	seen := make(map[string]struct{}, len(jobs))
	for _, job := range jobs {
		if job == nil || job.ID() == "" {
			return nil, ErrInvalidRunner
		}
		if _, exists := seen[job.ID()]; exists {
			return nil, ErrDuplicateJob
		}
		seen[job.ID()] = struct{}{}
	}
	return &Runner{jobs: append([]Job(nil), jobs...)}, nil
}

func (runner *Runner) RunOnce(ctx context.Context) ([]Result, error) {
	if runner == nil || ctx == nil {
		return nil, ErrInvalidRunner
	}
	results := make([]Result, 0, len(runner.jobs))
	var firstErr error
	for _, job := range runner.jobs {
		err := job.Run(ctx)
		results = append(results, Result{ID: job.ID(), Err: err})
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return results, firstErr
}

type FuncJob struct {
	Name    string
	Handler func(context.Context) error
}

func (job FuncJob) ID() string { return job.Name }
func (job FuncJob) Run(ctx context.Context) error {
	if job.Handler == nil {
		return ErrInvalidRunner
	}
	return job.Handler(ctx)
}
