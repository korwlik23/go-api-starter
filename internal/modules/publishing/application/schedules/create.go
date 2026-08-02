package schedules

import (
	"context"
	"errors"
	"strings"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
)

func (service *Service) Create(ctx context.Context, input CreateInput) (publishingdomain.PublicationSchedule, error) {
	if service == nil || ctx == nil || strings.TrimSpace(input.AccountID) == "" ||
		strings.TrimSpace(input.ActorUserID) == "" || strings.TrimSpace(input.RequestID) == "" ||
		strings.TrimSpace(input.TranslationID) == "" || input.ExpectedVersion < 1 {
		return publishingdomain.PublicationSchedule{}, ErrInvalidScheduleCommand
	}
	existing, err := service.dependencies.Repository.FindByIdempotency(ctx, input.AccountID, input.IdempotencyKey)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, ports.ErrScheduleNotFound) {
		return publishingdomain.PublicationSchedule{}, err
	}
	now := service.dependencies.Now().UTC()
	if now.IsZero() {
		return publishingdomain.PublicationSchedule{}, ErrInvalidScheduleCommand
	}
	id, err := service.dependencies.NewID()
	if err != nil {
		return publishingdomain.PublicationSchedule{}, err
	}
	schedule, err := publishingdomain.NewPublicationSchedule(publishingdomain.PublicationScheduleInput{
		ID: id, AccountID: input.AccountID, TranslationID: input.TranslationID,
		PublishAt: input.PublishAt, UnpublishAt: input.UnpublishAt,
		ExpectedVersion: input.ExpectedVersion, IdempotencyKey: input.IdempotencyKey,
		State: publishingdomain.SchedulePending,
	})
	if err != nil {
		return publishingdomain.PublicationSchedule{}, ErrInvalidScheduleCommand
	}
	operation := func(operationContext context.Context) error {
		if err := service.dependencies.Repository.Create(operationContext, ports.ScheduleCreateCommand{Schedule: schedule}); err != nil {
			return err
		}
		return service.dependencies.Auditor.RecordScheduleChange(operationContext, ScheduleAuditEvent{
			ActorUserID: input.ActorUserID, AccountID: input.AccountID, RequestID: input.RequestID,
			Action: "publishing.schedule.created", ScheduleID: schedule.ID,
			TranslationID: schedule.TranslationID, State: schedule.State,
		})
	}
	if err := service.runMutation(ctx, operation); err != nil {
		return publishingdomain.PublicationSchedule{}, err
	}
	return schedule, nil
}
