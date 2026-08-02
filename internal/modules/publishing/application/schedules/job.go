package schedules

import "context"

// WorkerJob adapts the publishing scheduler to the platform job runner. The
// application bootstrap supplies the configured Service and limit.
type WorkerJob struct {
	Service *Service
	Limit   int
}

func (job WorkerJob) ID() string { return "publishing.schedule.process" }

func (job WorkerJob) Run(ctx context.Context) error {
	if job.Service == nil || job.Limit < 1 {
		return ErrInvalidScheduleService
	}
	_, err := job.Service.RunDue(ctx, RunDueInput{Limit: job.Limit})
	return err
}
