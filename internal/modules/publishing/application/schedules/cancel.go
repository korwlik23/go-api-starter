package schedules

import (
	"context"
	"strings"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
)

func (service *Service) Cancel(ctx context.Context, input CancelInput) error {
	if service == nil || ctx == nil || strings.TrimSpace(input.AccountID) == "" ||
		strings.TrimSpace(input.ActorUserID) == "" || strings.TrimSpace(input.RequestID) == "" ||
		strings.TrimSpace(input.ScheduleID) == "" {
		return ErrInvalidScheduleCommand
	}
	now := service.dependencies.Now().UTC()
	if now.IsZero() {
		return ErrInvalidScheduleCommand
	}
	return service.runMutation(ctx, func(operationContext context.Context) error {
		if err := service.dependencies.Repository.Cancel(operationContext, ports.ScheduleCancelCommand{
			AccountID: input.AccountID, ScheduleID: input.ScheduleID, CanceledAt: now,
		}); err != nil {
			return err
		}
		return service.dependencies.Auditor.RecordScheduleChange(operationContext, ScheduleAuditEvent{
			ActorUserID: input.ActorUserID, AccountID: input.AccountID, RequestID: input.RequestID,
			Action: "publishing.schedule.canceled", ScheduleID: input.ScheduleID,
			State: publishingdomain.ScheduleCanceled,
		})
	})
}
