package workflow

import (
	"context"
	"strings"

	authdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing"
	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
)

func (service *Service) Transition(
	ctx context.Context,
	input TransitionInput,
) (result ports.ContentDetail, resultErr error) {
	if service == nil || ctx == nil ||
		strings.TrimSpace(input.AccountID) == "" ||
		strings.TrimSpace(input.ActorUserID) == "" ||
		strings.TrimSpace(input.RequestID) == "" ||
		strings.TrimSpace(input.ContentID) == "" ||
		strings.TrimSpace(input.TranslationID) == "" ||
		strings.TrimSpace(input.LocaleID) == "" ||
		input.ExpectedVersion < 1 {
		return ports.ContentDetail{}, ErrInvalidWorkflowCommand
	}
	current, err := service.dependencies.Repository.Get(
		ctx, input.AccountID, input.ContentID, input.LocaleID,
	)
	if err != nil {
		return ports.ContentDetail{}, err
	}
	if current.TranslationID != input.TranslationID ||
		current.TranslationVersion != input.ExpectedVersion {
		return ports.ContentDetail{}, ports.ErrWorkflowConflict
	}
	target, permission, action, err := transitionTarget(input.Action, current.TranslationStatus)
	if err != nil {
		return ports.ContentDetail{}, err
	}
	if allowed, authErr := service.dependencies.Authorizer.Allowed(
		ctx, input.ActorUserID, input.AccountID, permission, input.ContentID,
	); authErr != nil {
		return ports.ContentDetail{}, authErr
	} else if !allowed {
		return ports.ContentDetail{}, ErrWorkflowDenied
	}
	if _, err := publishingdomain.Transition(current.TranslationStatus, target); err != nil {
		return ports.ContentDetail{}, ports.ErrWorkflowConflict
	}
	now := service.dependencies.Now().UTC()
	if now.IsZero() {
		return ports.ContentDetail{}, ErrInvalidWorkflowCommand
	}
	eventID, err := service.dependencies.NewID()
	if err != nil {
		return ports.ContentDetail{}, err
	}
	operation := func(operationContext context.Context) error {
		result, err = service.dependencies.Repository.Transition(
			operationContext,
			ports.WorkflowTransitionCommand{
				AccountID: input.AccountID, ContentID: input.ContentID,
				TranslationID: input.TranslationID, LocaleID: input.LocaleID,
				ExpectedVersion: input.ExpectedVersion,
				FromStatus:      current.TranslationStatus, ToStatus: target, UpdatedAt: now,
			},
		)
		if err != nil {
			return err
		}
		if err := service.dependencies.Repository.AppendEvent(
			operationContext,
			ports.WorkflowEvent{
				ID: eventID, AccountID: input.AccountID, ContentID: input.ContentID,
				TranslationID: input.TranslationID, ActorID: input.ActorUserID,
				FromStatus: current.TranslationStatus, ToStatus: target,
				RequestID: input.RequestID, OccurredAt: now,
			},
		); err != nil {
			return err
		}
		return service.dependencies.Auditor.RecordWorkflowChange(
			operationContext,
			WorkflowAuditEvent{
				ActorUserID: input.ActorUserID, AccountID: input.AccountID,
				RequestID: input.RequestID, Action: action,
				ContentID: input.ContentID, TranslationID: input.TranslationID,
				FromStatus: string(current.TranslationStatus), ToStatus: string(target),
				Version: input.ExpectedVersion + 1,
			},
		)
	}
	resultErr = service.runMutation(ctx, operation)
	return result, resultErr
}

func transitionTarget(
	action string,
	current publishingdomain.Status,
) (publishingdomain.Status, authdomain.PermissionKey, string, error) {
	var permissionRaw string
	var target publishingdomain.Status
	switch action {
	case "submit_review":
		target, permissionRaw = publishingdomain.StatusReview, publishing.PermissionPagesWriteOwn
	case "approve":
		target, permissionRaw = publishingdomain.StatusPublished, publishing.PermissionPagesReviewAny
	case "publish":
		target, permissionRaw = publishingdomain.StatusPublished, publishing.PermissionPagesPublishAny
	case "schedule":
		target, permissionRaw = publishingdomain.StatusScheduled, publishing.PermissionPagesPublishAny
	case "archive":
		target, permissionRaw = publishingdomain.StatusArchived, publishing.PermissionPagesPublishAny
	default:
		return "", authdomain.PermissionKey{}, "", ErrInvalidWorkflowCommand
	}
	permission, err := authdomain.ParsePermissionKey(permissionRaw)
	if err != nil {
		return "", authdomain.PermissionKey{}, "", ErrInvalidWorkflowService
	}
	if _, err := publishingdomain.Transition(current, target); err != nil {
		return "", authdomain.PermissionKey{}, "", ports.ErrWorkflowConflict
	}
	return target, permission, "publishing.workflow." + action, nil
}
