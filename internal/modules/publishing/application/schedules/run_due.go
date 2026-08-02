package schedules

import (
	"context"
	"strings"
	"time"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
)

func (service *Service) RunDue(ctx context.Context, input RunDueInput) (RunDueResult, error) {
	if service == nil || ctx == nil || input.Limit < 1 || input.Limit > 100 {
		return RunDueResult{}, ErrInvalidScheduleCommand
	}
	now := service.dependencies.Now().UTC()
	if now.IsZero() {
		return RunDueResult{}, ErrInvalidScheduleCommand
	}
	schedules, err := service.dependencies.Repository.ClaimDue(ctx, now, now.Add(service.dependencies.LeaseDuration), input.Limit)
	if err != nil {
		return RunDueResult{}, err
	}
	result := RunDueResult{Claimed: len(schedules)}
	for _, schedule := range schedules {
		executeErr := service.dependencies.Executor.Execute(ctx, schedule)
		if executeErr == nil {
			finishErr := service.dependencies.Repository.Finish(ctx, ports.ScheduleFinishCommand{
				AccountID: schedule.AccountID, ScheduleID: schedule.ID, ExpectedAttempts: schedule.Attempts,
				State: publishingdomain.ScheduleSucceeded, FinishedAt: now,
			})
			if finishErr != nil {
				return result, finishErr
			}
			result.Succeeded++
			if err := service.dependencies.Auditor.RecordScheduleChange(ctx, ScheduleAuditEvent{
				AccountID: schedule.AccountID, Action: "publishing.schedule.succeeded",
				ScheduleID: schedule.ID, TranslationID: schedule.TranslationID,
				State: publishingdomain.ScheduleSucceeded, Attempts: schedule.Attempts,
			}); err != nil {
				return result, err
			}
			continue
		}
		state := publishingdomain.SchedulePending
		result.Retried++
		if schedule.Attempts >= service.dependencies.MaxAttempts {
			state = publishingdomain.ScheduleFailed
			result.Retried--
			result.Failed++
		}
		reason := sanitizeFailure(executeErr)
		var next *time.Time
		if state == publishingdomain.SchedulePending {
			candidate := now.Add(backoff(schedule.Attempts))
			next = &candidate
		}
		if err := service.dependencies.Repository.Finish(ctx, ports.ScheduleFinishCommand{
			AccountID: schedule.AccountID, ScheduleID: schedule.ID, ExpectedAttempts: schedule.Attempts,
			State: state, FailureReason: reason, NextAttemptAt: next, FinishedAt: now,
		}); err != nil {
			return result, err
		}
		if err := service.dependencies.Auditor.RecordScheduleChange(ctx, ScheduleAuditEvent{
			AccountID: schedule.AccountID, Action: "publishing.schedule.failed",
			ScheduleID: schedule.ID, TranslationID: schedule.TranslationID,
			State: state, Attempts: schedule.Attempts,
		}); err != nil {
			return result, err
		}
	}
	return result, nil
}

func backoff(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	if attempts > 6 {
		attempts = 6
	}
	return time.Duration(1<<uint(attempts-1)) * time.Minute
}

func sanitizeFailure(err error) string {
	if err == nil {
		return ""
	}
	value := strings.TrimSpace(err.Error())
	value = strings.ReplaceAll(strings.ReplaceAll(value, "\r", " "), "\n", " ")
	if len(value) > 2000 {
		return value[:2000]
	}
	return value
}
